import * as client from 'openid-client'

import { serverEnv } from '#/server/runtime/env'

// one Keycloak realm per tenant (ADR 0010), so discovery is per slug
const realmConfigBySlug = new Map<string, Promise<client.Configuration>>()

export const realmConfig = (slug: string): Promise<client.Configuration> => {
  if (!/^[a-z0-9][a-z0-9-]{1,62}$/.test(slug)) return Promise.reject(new Error('invalid tenant slug'))
  let discovery = realmConfigBySlug.get(slug)
  if (!discovery) {
    const env = serverEnv()
    const issuer = new URL(`${env.keycloakUrl}/realms/${slug}`)
    const discoveryOptions: client.DiscoveryRequestOptions = {}
    // dev Keycloak is plain http; openid-client refuses that unless told so
    if (issuer.protocol === 'http:') discoveryOptions.execute = [client.allowInsecureRequests]
    discovery = client.discovery(issuer, env.clientId, env.clientSecret, undefined, discoveryOptions)
    discovery.catch(() => realmConfigBySlug.delete(slug))
    realmConfigBySlug.set(slug, discovery)
  }
  return discovery
}

// the dispute-api scope already carries username, email, roles and tenant_id
export const scopes = 'openid'
