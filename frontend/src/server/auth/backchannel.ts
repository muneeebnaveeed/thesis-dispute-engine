import { createRemoteJWKSet, decodeJwt, jwtVerify, type JWTPayload, type JWTVerifyGetKey } from 'jose'
import createClient from 'openapi-fetch'

import type { paths } from '#/api/schema.gen'
import { serverEnv } from '../env'
import { realmConfig } from './oidc'

const jwksByIssuer = new Map<string, ReturnType<typeof createRemoteJWKSet>>()

/** Which sessions a logout token names: the realm session, or every session of the subject at that tenant. */
export type LogoutTarget = { sid: string } | { issuer: string; subject: string }

type Deps = { jwks: (issuer: string, slug: string) => Promise<JWTVerifyGetKey> }

// OpenID Connect Back-Channel Logout 1.0: an id_token-like JWT with the logout event, sid and/or sub, no nonce.
// The issuer must be one of our realms: the slug is read from the token, the expected issuer is rebuilt from
// configuration, and the two must agree before any key is fetched.
export async function verifyLogoutToken(
  raw: string,
  deps: Deps = { jwks: jwksFor },
): Promise<LogoutTarget | null> {
  let claimedIssuer: string
  try {
    const iss = decodeJwt(raw).iss
    if (typeof iss !== 'string') return null
    claimedIssuer = iss
  } catch {
    return null
  }
  const slug = claimedIssuer.split('/realms/')[1]
  if (!slug || !/^[a-z0-9][a-z0-9-]{1,62}$/.test(slug)) return null
  const env = serverEnv()
  const issuer = `${env.keycloakUrl.replace(/\/$/, '')}/realms/${slug}`
  if (claimedIssuer !== issuer) return null
  try {
    const { payload } = await jwtVerify(raw, await deps.jwks(issuer, slug), {
      issuer,
      audience: env.clientId,
      maxTokenAge: '5m',
    })
    return targetOf(payload)
  } catch {
    return null
  }
}

function targetOf(p: JWTPayload): LogoutTarget | null {
  const events = p.events
  if (
    !events ||
    typeof events !== 'object' ||
    !('http://schemas.openid.net/event/backchannel-logout' in events)
  )
    return null
  if ('nonce' in p) return null
  if (typeof p.sid === 'string' && p.sid) return { sid: p.sid }
  if (typeof p.sub === 'string' && p.sub && typeof p.iss === 'string')
    return { issuer: p.iss, subject: p.sub }
  return null
}

async function jwksFor(issuer: string, slug: string) {
  let set = jwksByIssuer.get(issuer)
  if (!set) {
    const config = await realmConfig(slug)
    const uri = config.serverMetadata().jwks_uri
    if (!uri) throw new Error('realm has no jwks_uri')
    set = createRemoteJWKSet(new URL(uri))
    jwksByIssuer.set(issuer, set)
  }
  return set
}

/** Ends the sessions a valid logout token names through the API's internal endpoint. */
export async function handleBackchannelLogout(raw: string): Promise<boolean> {
  const target = await verifyLogoutToken(raw)
  if (!target) return false
  const env = serverEnv()
  const api = createClient<paths>({ baseUrl: env.apiUrl })
  const headers = { 'X-Service-Key': env.serviceKey }
  if ('sid' in target) {
    const { response } = await api.DELETE('/internal/sessions', {
      params: { query: { sid: target.sid } },
      headers,
    })
    return response.ok
  }
  const { data } = await api.GET('/internal/tenants', { headers })
  const tenant = data?.find((t) => t.issuer === target.issuer)
  if (!tenant) return false
  const { response } = await api.DELETE('/internal/sessions', {
    params: { query: { tenantId: tenant.id, subject: target.subject } },
    headers,
  })
  return response.ok
}
