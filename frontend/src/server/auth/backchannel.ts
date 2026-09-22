import { createRemoteJWKSet, decodeJwt, jwtVerify, type JWTPayload, type JWTVerifyGetKey } from 'jose'
import createClient from 'openapi-fetch'

import type { paths } from '#/api/schema.gen'
import { serverEnv } from '#/server/runtime/env'
import { realmConfig } from './oidc'

const jwksByIssuer = new Map<string, ReturnType<typeof createRemoteJWKSet>>()

export type LogoutTarget = { sid: string } | { issuer: string; subject: string }

type KeySource = { jwks: (issuer: string, slug: string) => Promise<JWTVerifyGetKey> }

// the slug comes from the token but the issuer is rebuilt from configuration; both must agree before any key is fetched
export const verifyLogoutToken = async (
  logoutToken: string,
  keys: KeySource = { jwks: jwksFor },
): Promise<LogoutTarget | null> => {
  let claimedIssuer: string
  try {
    const claimedIss = decodeJwt(logoutToken).iss
    if (typeof claimedIss !== 'string') return null
    claimedIssuer = claimedIss
  } catch {
    return null
  }
  const slug = claimedIssuer.split('/realms/')[1]
  if (!slug || !/^[a-z0-9][a-z0-9-]{1,62}$/.test(slug)) return null
  const env = serverEnv()
  const expectedIssuer = `${env.keycloakUrl.replace(/\/$/, '')}/realms/${slug}`
  if (claimedIssuer !== expectedIssuer) return null
  try {
    const { payload } = await jwtVerify(logoutToken, await keys.jwks(expectedIssuer, slug), {
      issuer: expectedIssuer,
      audience: env.clientId,
      maxTokenAge: '5m',
    })
    return logoutTargetOf(payload)
  } catch {
    return null
  }
}

const BACKCHANNEL_LOGOUT_EVENT = 'http://schemas.openid.net/event/backchannel-logout'

const logoutTargetOf = (claims: JWTPayload): LogoutTarget | null => {
  const events = claims.events
  if (!events || typeof events !== 'object' || !(BACKCHANNEL_LOGOUT_EVENT in events)) return null
  if ('nonce' in claims) return null
  if (typeof claims.sid === 'string' && claims.sid) return { sid: claims.sid }
  if (typeof claims.sub === 'string' && claims.sub && typeof claims.iss === 'string') {
    return { issuer: claims.iss, subject: claims.sub }
  }
  return null
}

const jwksFor = async (issuer: string, slug: string) => {
  let keySet = jwksByIssuer.get(issuer)
  if (!keySet) {
    const config = await realmConfig(slug)
    const jwksUri = config.serverMetadata().jwks_uri
    if (!jwksUri) throw new Error('realm has no jwks_uri')
    keySet = createRemoteJWKSet(new URL(jwksUri))
    jwksByIssuer.set(issuer, keySet)
  }
  return keySet
}

export const handleBackchannelLogout = async (logoutToken: string): Promise<boolean> => {
  const target = await verifyLogoutToken(logoutToken)
  if (!target) return false
  const env = serverEnv()
  const internalApi = createClient<paths>({ baseUrl: env.apiUrl })
  const headers = { 'X-Service-Key': env.serviceKey }
  if ('sid' in target) {
    const { response } = await internalApi.DELETE('/internal/sessions', {
      params: { query: { sid: target.sid } },
      headers,
    })
    return response.ok
  }
  const { data: tenants } = await internalApi.GET('/internal/tenants', { headers })
  const tenant = tenants?.find((candidate) => candidate.issuer === target.issuer)
  if (!tenant) return false
  const { response } = await internalApi.DELETE('/internal/sessions', {
    params: { query: { tenantId: tenant.id, subject: target.subject } },
    headers,
  })
  return response.ok
}
