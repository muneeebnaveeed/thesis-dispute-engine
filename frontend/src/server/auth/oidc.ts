import * as client from 'openid-client'

import { serverEnv } from '#/server/runtime/env'

// One Keycloak realm per tenant (ADR 0010): the slug names the realm, so discovery is per slug and cached.
const configs = new Map<string, Promise<client.Configuration>>()

export const realmConfig = (slug: string): Promise<client.Configuration> => {
  if (!/^[a-z0-9][a-z0-9-]{1,62}$/.test(slug)) return Promise.reject(new Error('invalid tenant slug'))
  let p = configs.get(slug)
  if (!p) {
    const env = serverEnv()
    const issuer = new URL(`${env.keycloakUrl}/realms/${slug}`)
    const opts: client.DiscoveryRequestOptions = {}
    // Dev Keycloak is plain http on localhost; openid-client refuses that unless told the deployment is local.
    if (issuer.protocol === 'http:') opts.execute = [client.allowInsecureRequests]
    p = client.discovery(issuer, env.clientId, env.clientSecret, undefined, opts)
    p.catch(() => configs.delete(slug))
    configs.set(slug, p)
  }
  return p
}

// The realm's dispute-api scope already carries username, email, roles and tenant_id; nothing else is requested.
export const scopes = 'openid'
