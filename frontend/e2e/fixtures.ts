import { expect, test as base, type APIRequestContext, type Page } from '@playwright/test'

// The seeded world from cmd/seed and deploy/keycloak/render-realms.sh.
export const api = process.env.E2E_API_URL ?? 'http://localhost:8090'
export const tenants = {
  otp: { slug: 'otp', key: 'tk_dev_tenant_a', transaction: '00000000-0000-8000-8000-000000000102' },
  erste: { slug: 'erste', key: 'tk_dev_tenant_b', transaction: '00000000-0000-8000-8000-000000000301' },
}
export const analyst = { username: 'analyst', password: 'analyst' }

/** Opens a dispute the way a tenant's own system would, so tests have something to protect and act on. */
export async function openDisputeViaApi(
  request: APIRequestContext,
  tenant: keyof typeof tenants,
): Promise<string> {
  const t = tenants[tenant]
  const res = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${t.key}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: t.transaction, actor: 'e2e' },
  })
  expect(res.status(), await res.text()).toBe(201)
  const body: unknown = await res.json()
  if (!body || typeof body !== 'object' || typeof (body as { id?: unknown }).id !== 'string')
    throw new Error('no id in response')
  return (body as { id: string }).id
}

/** Fills Keycloak's login page if the browser is on it; a live realm SSO session skips it entirely. */
export async function completeKeycloakLogin(page: Page) {
  await page.waitForURL(/\/protocol\/openid-connect\/|localhost:3002/)
  if (page.url().includes('/protocol/openid-connect/')) {
    await page.fill('#username', analyst.username)
    await page.fill('#password', analyst.password)
    await page.click('#kc-login')
  }
  await page.waitForURL(/localhost:3002\//)
}

/** Visits the tenant's front door and signs in; the app never asks for the tenant. */
export async function signIn(page: Page, slug: string, path = `/${slug}`) {
  await page.goto(path)
  await completeKeycloakLogin(page)
}

export const test = base
export { expect }
