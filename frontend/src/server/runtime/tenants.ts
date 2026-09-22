import createClient from 'openapi-fetch'

import type { components, paths } from '#/api/schema.gen'
import { serverEnv } from '#/server/runtime/env'

export type TenantSummary = components['schemas']['TenantSummary']

// cached briefly: tenants change at onboarding time, not per request
const DIRECTORY_CACHE_MILLIS = 60_000
let directoryCache: { fetchedAt: number; tenants: TenantSummary[] } | null = null

const activeTenants = async (): Promise<TenantSummary[]> => {
  if (directoryCache && Date.now() - directoryCache.fetchedAt < DIRECTORY_CACHE_MILLIS)
    return directoryCache.tenants
  const env = serverEnv()
  const internalApi = createClient<paths>({ baseUrl: env.apiUrl })
  const { data: tenants, response } = await internalApi.GET('/internal/tenants', {
    headers: { 'X-Service-Key': env.serviceKey },
  })
  if (!response.ok || !tenants) throw new Error(`tenant directory: ${response.status}`)
  directoryCache = { fetchedAt: Date.now(), tenants }
  return tenants
}

export const tenantBySlug = async (slug: string): Promise<TenantSummary | null> =>
  (await activeTenants()).find((tenant) => tenant.slug === slug) ?? null

export const tenantByEmail = async (email: string): Promise<TenantSummary | null> => {
  const atSign = email.lastIndexOf('@')
  if (atSign < 1) return null
  const emailDomain = email
    .slice(atSign + 1)
    .trim()
    .toLowerCase()
  if (!emailDomain) return null
  return (
    (await activeTenants()).find((tenant) =>
      tenant.emailDomains.some((domain) => domain.toLowerCase() === emailDomain),
    ) ?? null
  )
}
