import createClient from 'openapi-fetch'

import type { components, paths } from '#/api/schema.gen'
import { serverEnv } from './env'

export type TenantSummary = components['schemas']['TenantSummary']

// The active tenant directory from the API, cached briefly: it changes at onboarding time, not per request.
let cache: { at: number; list: TenantSummary[] } | null = null

export async function activeTenants(): Promise<TenantSummary[]> {
  if (cache && Date.now() - cache.at < 60_000) return cache.list
  const env = serverEnv()
  const client = createClient<paths>({ baseUrl: env.apiUrl })
  const { data, response } = await client.GET('/internal/tenants', {
    headers: { 'X-Service-Key': env.serviceKey },
  })
  if (!response.ok || !data) throw new Error(`tenant directory: ${response.status}`)
  cache = { at: Date.now(), list: data }
  return data
}

export async function tenantBySlug(slug: string): Promise<TenantSummary | null> {
  return (await activeTenants()).find((t) => t.slug === slug) ?? null
}

/** Home-realm discovery: the domain of a work email names the tenant. */
export async function tenantByEmail(email: string): Promise<TenantSummary | null> {
  const at = email.lastIndexOf('@')
  if (at < 1) return null
  const domain = email
    .slice(at + 1)
    .trim()
    .toLowerCase()
  if (!domain) return null
  return (await activeTenants()).find((t) => t.emailDomains.some((d) => d.toLowerCase() === domain)) ?? null
}
