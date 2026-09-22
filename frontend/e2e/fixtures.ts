import { expect, test as base, type APIRequestContext, type Page } from '@playwright/test'

export const api = process.env.E2E_API_URL ?? 'http://localhost:8090'
export const tenants = {
  otp: { slug: 'otp', key: 'tk_dev_tenant_a', transaction: '00000000-0000-8000-8000-000000000102' },
  erste: { slug: 'erste', key: 'tk_dev_tenant_b', transaction: '00000000-0000-8000-8000-000000000301' },
}
export const analyst = { username: 'analyst', password: 'analyst' }

export const openDisputeViaApi = async (
  request: APIRequestContext,
  tenant: keyof typeof tenants,
): Promise<string> => {
  const seeded = tenants[tenant]
  const response = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${seeded.key}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: seeded.transaction, actor: 'e2e' },
  })
  expect(response.status(), await response.text()).toBe(201)
  const body: unknown = await response.json()
  if (!body || typeof body !== 'object' || typeof (body as { id?: unknown }).id !== 'string')
    throw new Error('no id in response')
  return (body as { id: string }).id
}

export const completeKeycloakLogin = async (page: Page) => {
  await page.waitForURL(/\/protocol\/openid-connect\/|localhost:3002/)
  if (page.url().includes('/protocol/openid-connect/')) {
    await page.fill('#username', analyst.username)
    await page.fill('#password', analyst.password)
    await page.click('#kc-login')
  }
  // any localhost:3002 URL would match the callback itself and cut session creation short
  await page.waitForURL((url) => url.host === 'localhost:3002' && !url.pathname.startsWith('/auth/'))
}

export const signIn = async (page: Page, slug: string, path = `/${slug}`) => {
  await page.goto(path)
  await completeKeycloakLogin(page)
}

export const test = base
export { expect }
