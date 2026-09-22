import { completeKeycloakLogin, expect, openDisputeViaApi, signIn, test } from './fixtures'

test('a signed-out visitor to a tenant page is sent through the realm and back to that page', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}`)
  // No tenant chooser, no slug to type: straight to the realm's own login.
  await page.waitForURL(/\/realms\/otp\/protocol\/openid-connect\//)
  await completeKeycloakLogin(page)

  await expect(page).toHaveURL(new RegExp(`/otp/disputes/${id}$`))
  await expect(page.getByRole('heading', { name: `Dispute ${id.slice(0, 8)}` })).toBeVisible()
  await expect(page.getByText('OTP Bank')).toBeVisible()

  const cookies = await page.context().cookies()
  const session = cookies.find((cookie) => cookie.name === 'de_session')
  expect(session?.httpOnly).toBe(true)
  expect(session?.value).toMatch(/^[0-9a-f-]{36}$/)
  expect(cookies.find((cookie) => cookie.name === 'de_tenant')?.value).toBe('otp')
  const stored = await page.evaluate(() =>
    [...Object.values(localStorage), ...Object.values(sessionStorage)].join(' '),
  )
  expect(stored).not.toMatch(/eyJ[A-Za-z0-9_-]+\.|Bearer |tk_/)
})

test('an action from a tab whose session has ended signs in again and lands back on the page', async ({
  page,
  context,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await signIn(page, 'otp', `/otp/disputes/${id}`)
  await expect(page.getByRole('button', { name: 'OPEN_INVESTIGATION' })).toBeVisible()

  // only the app's cookies go; the realm's SSO cookies (path /realms/...) stay, so sign-in is silent
  const realmCookies = (await context.cookies()).filter((cookie) => cookie.path.startsWith('/realms'))
  await context.clearCookies()
  await context.addCookies(realmCookies)
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()

  await expect(page).toHaveURL(new RegExp(`/otp/disputes/${id}$`))
  await expect(page.getByRole('heading', { name: `Dispute ${id.slice(0, 8)}` })).toBeVisible()
  await expect(page.getByText('OTP Bank')).toBeVisible()
})

test('the root remembers the tenant, and a work email finds it for a fresh browser', async ({
  page,
  context,
}) => {
  await signIn(page, 'otp')
  await page.goto('/')
  await expect(page).toHaveURL(/\/otp$/)

  await context.clearCookies()
  await page.goto('/')
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()
  await page.getByLabel('Work email').fill('someone@otpbank.hu')
  // Cookies are gone, so Keycloak must show its form; waiting for it keeps the login helper from seeing '/' as done.
  await Promise.all([
    page.waitForURL(/\/protocol\/openid-connect\/auth/),
    page.getByRole('button', { name: 'Continue' }).click(),
  ])
  await completeKeycloakLogin(page)
  await expect(page).toHaveURL(/\/otp$/)

  await context.clearCookies()
  await page.goto('/')
  await page.getByLabel('Work email').fill('someone@unknown.example')
  await page.getByRole('button', { name: 'Continue' }).click()
  await expect(page.getByText(/could not find an organisation/)).toBeVisible()
})

test('sign-out ends the session, lands on the tenant door, and the next visit needs the realm again', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await signIn(page, 'otp')
  await page.getByRole('button', { name: 'Sign out' }).click()
  // Keycloak ends its SSO, sends the browser to the tenant door, and the door leads to a fresh login page.
  await page.waitForURL(/\/realms\/otp\/protocol\/openid-connect\/auth/)
  await expect(page.locator('#username')).toBeVisible()
  await page.goto(`/otp/disputes/${id}`)
  await page.waitForURL(/\/realms\/otp\/protocol\/openid-connect\//)
  await expect(page.locator('#username')).toBeVisible()
})

test('a destination outside this tenant is never used after sign-in', async ({ page }) => {
  await page.goto('/otp?next=https%3A%2F%2Fevil.example%2F')
  await completeKeycloakLogin(page)
  await expect(page).toHaveURL(/localhost:3002\/otp$/)
  await page.goto('/otp?next=%2Ferste%2Fdisputes%2Fx')
  await expect(page).toHaveURL(/localhost:3002\/otp$/)
})

test('every page carries the security headers and a nonce-based CSP', async ({ page, request }) => {
  const response = await page.goto('/')
  const csp = response?.headers()['content-security-policy'] ?? ''
  expect(csp).toMatch(/script-src 'self' 'nonce-[a-f0-9]{32}'/)
  expect(csp).toContain("frame-ancestors 'none'")
  expect(response?.headers()['x-content-type-options']).toBe('nosniff')
  // Start's inline scripts remove themselves after running, so check the raw HTML: each carries the header's nonce.
  const raw = await request.get('/')
  const html = await raw.text()
  const nonce = /'nonce-([a-f0-9]{32})'/.exec(raw.headers()['content-security-policy'] ?? '')?.[1]
  const scripts = html.match(/<script\b[^>]*>/g) ?? []
  expect(scripts.length).toBeGreaterThan(0)
  for (const tag of scripts) expect(tag).toContain(`nonce="${nonce}"`)
})
