// server-only; route files import functions/session.ts, never this
import { deleteCookie, getCookie, setCookie } from '@tanstack/react-start/server'
import * as client from 'openid-client'

import { serverEnv } from '#/server/runtime/env'
import { safeNext } from './next'
import { realmConfig, scopes } from './oidc'
import * as store from './store'

const sessionCookie = 'de_session'
const REFRESH_AHEAD_MILLIS = 30_000
const LAST_SEEN_WRITE_INTERVAL_MILLIS = 60_000
const PENDING_LOGIN_SECONDS = 600
const REMEMBERED_TENANT_SECONDS = 365 * 24 * 3600
// non-secret: lets the root skip the tenant question
export const rememberedTenantCookie = 'de_tenant'

const cookieOptions = (maxAge = serverEnv().sessionTtlSeconds) => {
  return { httpOnly: true, sameSite: 'lax' as const, secure: serverEnv().secureCookies, path: '/', maxAge }
}

// never the tokens
export type Viewer = {
  tenantId: string
  tenantSlug: string
  name: string
  email: string | null
  roles: string[]
}

type SignedInSession = {
  id: string
  session: store.Session & { identity: NonNullable<store.Session['identity']> }
}

// sliding idle rule; last-seen advances at most once a minute to keep writes rare
const signedInSession = async (): Promise<SignedInSession | null> => {
  const sessionId = getCookie(sessionCookie)
  if (!sessionId) return null
  const session = await store.get(sessionId)
  if (!session?.identity) return null
  const env = serverEnv()
  const now = Date.now()
  const lastSeenAt = session.lastSeenAt ? Date.parse(session.lastSeenAt) : now
  if (now - lastSeenAt > env.sessionIdleSeconds * 1000) {
    await store.remove(sessionId)
    deleteCookie(sessionCookie, { path: '/' })
    return null
  }
  if (now - lastSeenAt > LAST_SEEN_WRITE_INTERVAL_MILLIS) {
    session.lastSeenAt = new Date(now).toISOString()
    await store.put(sessionId, session, env.sessionTtlSeconds)
  }
  return { id: sessionId, session: { ...session, identity: session.identity } }
}

export const viewer = async (): Promise<Viewer | null> => {
  const live = await signedInSession()
  if (!live) return null
  const { identity } = live.session
  return {
    tenantId: identity.tenantId,
    tenantSlug: live.session.tenantSlug,
    name: identity.name ?? identity.subject,
    email: identity.email,
    roles: identity.roles,
  }
}

export const begin = async ({ slug, next }: { slug: string; next?: string }): Promise<{ url: string }> => {
  const env = serverEnv()
  const config = await realmConfig(slug)
  const verifier = client.randomPKCECodeVerifier()
  const state = client.randomState()
  const nonce = client.randomNonce()
  const pendingId = crypto.randomUUID()
  await store.put(
    pendingId,
    { tenantSlug: slug, pending: { state, verifier, nonce, next: safeNext(next, slug) } },
    PENDING_LOGIN_SECONDS,
  )
  setCookie(sessionCookie, pendingId, cookieOptions(PENDING_LOGIN_SECONDS))
  const authorizationUrl = client.buildAuthorizationUrl(config, {
    redirect_uri: `${env.appUrl}/auth/callback`,
    scope: scopes,
    code_challenge: await client.calculatePKCECodeChallenge(verifier),
    code_challenge_method: 'S256',
    state,
    nonce,
  })
  return { url: authorizationUrl.href }
}

export const completeLogin = async (callbackUrl: URL): Promise<string> => {
  const env = serverEnv()
  const pendingId = getCookie(sessionCookie)
  if (!pendingId) throw new Error('no sign-in in progress')
  const pendingSession = await store.get(pendingId)
  if (!pendingSession?.pending) throw new Error('sign-in state expired')
  const { pending, tenantSlug } = pendingSession
  const config = await realmConfig(tenantSlug)
  const tokens = await client.authorizationCodeGrant(config, callbackUrl, {
    pkceCodeVerifier: pending.verifier,
    expectedState: pending.state,
    expectedNonce: pending.nonce,
  })
  const claims = tokens.claims()
  if (!claims) throw new Error('no id token')
  const tenantId = typeof claims.tenant_id === 'string' ? claims.tenant_id : null
  if (!tenantId) throw new Error('realm issued no tenant_id claim')
  const roles = Array.isArray(claims.roles)
    ? claims.roles.filter((role): role is string => typeof role === 'string')
    : []
  // fresh id: the pre-login cookie can never become a live session
  const sessionId = crypto.randomUUID()
  await store.put(
    sessionId,
    {
      tenantSlug,
      identity: {
        tenantId,
        subject: claims.sub,
        sid: typeof claims.sid === 'string' ? claims.sid : null,
        name: typeof claims.preferred_username === 'string' ? claims.preferred_username : null,
        email: typeof claims.email === 'string' ? claims.email : null,
        roles,
      },
      tokens: toTokens(tokens),
      lastSeenAt: new Date().toISOString(),
    },
    env.sessionTtlSeconds,
  )
  await store.remove(pendingId)
  setCookie(sessionCookie, sessionId, cookieOptions())
  setCookie(rememberedTenantCookie, tenantSlug, {
    ...cookieOptions(REMEMBERED_TENANT_SECONDS),
    httpOnly: true,
  })
  return pending.next
}

const toTokens = (granted: client.TokenEndpointResponse): NonNullable<store.Session['tokens']> => {
  const lifetimeSeconds = typeof granted.expires_in === 'number' ? granted.expires_in : 300
  return {
    access: granted.access_token,
    refresh: granted.refresh_token ?? null,
    id: granted.id_token ?? null,
    accessExpiresAt: new Date(Date.now() + lifetimeSeconds * 1000).toISOString(),
  }
}

export const accessTokenForRequest = async (): Promise<string | null> => {
  const live = await signedInSession()
  const tokens = live?.session.tokens
  if (!live || !tokens) return null
  const stillValidFor = Date.parse(tokens.accessExpiresAt) - Date.now()
  if (stillValidFor > REFRESH_AHEAD_MILLIS) return tokens.access
  if (!tokens.refresh) return null
  try {
    const config = await realmConfig(live.session.tenantSlug)
    const fresh = toTokens(await client.refreshTokenGrant(config, tokens.refresh))
    await store.put(
      live.id,
      { ...live.session, tokens: { ...fresh, id: fresh.id ?? tokens.id } },
      serverEnv().sessionTtlSeconds,
    )
    return fresh.access
  } catch {
    // refresh token dead (idle timeout, revoked): the session is over
    await store.remove(live.id)
    deleteCookie(sessionCookie, { path: '/' })
    return null
  }
}

export const endSession = async (): Promise<{ url: string }> => {
  const env = serverEnv()
  const live = await signedInSession()
  deleteCookie(sessionCookie, { path: '/' })
  if (!live) return { url: '/' }
  await store.remove(live.id)
  // the tenant's own front door, not the generic root
  const tenantHome = `${env.appUrl}/${live.session.tenantSlug}`
  try {
    const config = await realmConfig(live.session.tenantSlug)
    const endSessionParams: Record<string, string> = {
      post_logout_redirect_uri: tenantHome,
      client_id: env.clientId,
    }
    if (live.session.tokens?.id) endSessionParams.id_token_hint = live.session.tokens.id
    return { url: client.buildEndSessionUrl(config, endSessionParams).href }
  } catch {
    return { url: `/${live.session.tenantSlug}` }
  }
}
