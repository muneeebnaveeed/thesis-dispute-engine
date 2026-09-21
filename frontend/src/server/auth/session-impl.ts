// Server-only: cookies, OIDC, the sealed store. Route files never import this; session.ts wraps it in server functions.
import { deleteCookie, getCookie, setCookie } from '@tanstack/react-start/server'
import * as client from 'openid-client'

import { serverEnv } from '../env'
import { realmConfig, scopes } from './oidc'
import * as store from './store'

export const cookieName = 'de_session'

function cookieOptions(maxAge = serverEnv().sessionTtlSeconds) {
  return { httpOnly: true, sameSite: 'lax' as const, secure: serverEnv().secureCookies, path: '/', maxAge }
}

/** What the browser is allowed to know about the signed-in analyst. Never the tokens. */
export type Viewer = {
  tenantId: string
  tenantSlug: string
  name: string
  email: string | null
  roles: string[]
}

/** Only same-origin paths may be a post-login destination; anything else falls back to home. */
export function safeNext(raw: unknown): string {
  return typeof raw === 'string' && raw.startsWith('/') && !raw.startsWith('//') && !raw.startsWith('/t/')
    ? raw
    : '/'
}

type Live = { id: string; session: store.Session & { identity: NonNullable<store.Session['identity']> } }

async function current(): Promise<Live | null> {
  const id = getCookie(cookieName)
  if (!id) return null
  const session = await store.get(id)
  if (!session?.identity) return null
  return { id, session: { ...session, identity: session.identity } }
}

export async function viewer(): Promise<Viewer | null> {
  const live = await current()
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

/** Step 1 of login: remember state, PKCE and where to go afterwards, then hand the browser to the tenant's realm. */
export async function begin(data: { slug: string; next?: string }): Promise<{ url: string }> {
  const env = serverEnv()
  const config = await realmConfig(data.slug)
  const verifier = client.randomPKCECodeVerifier()
  const state = client.randomState()
  const nonce = client.randomNonce()
  const id = crypto.randomUUID()
  await store.put(
    id,
    { tenantSlug: data.slug, pending: { state, verifier, nonce, next: safeNext(data.next) } },
    600,
  )
  setCookie(cookieName, id, cookieOptions(600))
  const url = client.buildAuthorizationUrl(config, {
    redirect_uri: `${env.appUrl}/auth/callback`,
    scope: scopes,
    code_challenge: await client.calculatePKCECodeChallenge(verifier),
    code_challenge_method: 'S256',
    state,
    nonce,
  })
  return { url: url.href }
}

/** Step 2, run by /auth/callback with the full callback URL. Returns the path to send the browser to. */
export async function completeLogin(callbackUrl: URL): Promise<string> {
  const env = serverEnv()
  const pendingId = getCookie(cookieName)
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
    ? claims.roles.filter((r): r is string => typeof r === 'string')
    : []
  // A new id: the pre-login cookie value cannot be replayed into a live session.
  const id = crypto.randomUUID()
  await store.put(
    id,
    {
      tenantSlug,
      identity: {
        tenantId,
        subject: claims.sub,
        name: typeof claims.preferred_username === 'string' ? claims.preferred_username : null,
        email: typeof claims.email === 'string' ? claims.email : null,
        roles,
      },
      tokens: toTokens(tokens),
    },
    env.sessionTtlSeconds,
  )
  await store.remove(pendingId)
  setCookie(cookieName, id, cookieOptions())
  return pending.next
}

function toTokens(t: client.TokenEndpointResponse): NonNullable<store.Session['tokens']> {
  const ttl = typeof t.expires_in === 'number' ? t.expires_in : 300
  return {
    access: t.access_token,
    refresh: t.refresh_token ?? null,
    id: t.id_token ?? null,
    accessExpiresAt: new Date(Date.now() + ttl * 1000).toISOString(),
  }
}

/** The browser asks for this before calling the API directly; refresh happens here, never in the browser. */
export async function accessToken(): Promise<{ token: string; expiresAt: string } | null> {
  const live = await current()
  const tokens = live?.session.tokens
  if (!live || !tokens) return null
  if (Date.parse(tokens.accessExpiresAt) - Date.now() > 30_000) {
    return { token: tokens.access, expiresAt: tokens.accessExpiresAt }
  }
  if (!tokens.refresh) return null
  try {
    const config = await realmConfig(live.session.tenantSlug)
    const fresh = toTokens(await client.refreshTokenGrant(config, tokens.refresh))
    await store.put(
      live.id,
      { ...live.session, tokens: { ...fresh, id: fresh.id ?? tokens.id } },
      serverEnv().sessionTtlSeconds,
    )
    return { token: fresh.access, expiresAt: fresh.accessExpiresAt }
  } catch {
    // The refresh token died (idle timeout, revoked at Keycloak): the session is over.
    await store.remove(live.id)
    deleteCookie(cookieName, { path: '/' })
    return null
  }
}

/** Server-side callers (SSR loaders) get the same token the browser would, with the same refresh rule. */
export async function accessTokenForRequest(): Promise<string | null> {
  const res = await accessToken()
  return res?.token ?? null
}

export async function endSession(): Promise<{ url: string }> {
  const env = serverEnv()
  const live = await current()
  deleteCookie(cookieName, { path: '/' })
  if (!live) return { url: '/' }
  await store.remove(live.id)
  try {
    const config = await realmConfig(live.session.tenantSlug)
    const params: Record<string, string> = { post_logout_redirect_uri: env.appUrl, client_id: env.clientId }
    if (live.session.tokens?.id) params.id_token_hint = live.session.tokens.id
    return { url: client.buildEndSessionUrl(config, params).href }
  } catch {
    return { url: '/' }
  }
}
