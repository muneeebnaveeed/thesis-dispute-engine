import { expect, openDisputeViaApi, signIn, test } from './fixtures'

test('a signed-out visitor is sent to sign-in and returned to the page they asked for', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')

  await page.goto(`/disputes/${id}`)
  await expect(page).toHaveURL(
    new RegExp(`/\\?next=${encodeURIComponent(`/disputes/${id}`).replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`),
  )
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()

  // The demo link carries next along; Keycloak's own page does the authentication.
  await page.getByRole('link', { name: 'OTP Bank' }).click()
  await page.waitForURL(/\/realms\/otp\/protocol\/openid-connect\//)
  await page.fill('#username', 'analyst')
  await page.fill('#password', 'analyst')
  await page.click('#kc-login')

  await expect(page).toHaveURL(new RegExp(`/disputes/${id}$`))
  await expect(page.getByRole('heading', { name: `Dispute ${id.slice(0, 8)}` })).toBeVisible()
  await expect(page.getByText(/analyst/)).toBeVisible()
  await expect(page.getByText(/at otp/)).toBeVisible()

  // The browser never received a token or a readable session; only the opaque cookie.
  const cookies = await page.context().cookies()
  const session = cookies.find((c) => c.name === 'de_session')
  expect(session?.httpOnly).toBe(true)
  expect(session?.value).toMatch(/^[0-9a-f-]{36}$/)
  // Web storage may hold router bookkeeping (scroll positions), never anything token-shaped.
  const stored = await page.evaluate(() =>
    [...Object.values(localStorage), ...Object.values(sessionStorage)].join(' '),
  )
  expect(stored).not.toMatch(/eyJ[A-Za-z0-9_-]+\.|Bearer |tk_/)
})

test('sign-out ends the session and protected pages redirect again', async ({ page, request }) => {
  const id = await openDisputeViaApi(request, 'otp')
  await signIn(page, 'otp')
  await expect(page.getByRole('heading', { name: 'Disputes' })).toBeVisible()

  await page.getByRole('button', { name: 'Sign out' }).click()
  await page.waitForURL(/localhost:3002\/?(\?.*)?$/)
  await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible()

  await page.goto(`/disputes/${id}`)
  await expect(page).toHaveURL(/\/\?next=/)
})

test('a destination outside this app is never used after sign-in', async ({ page }) => {
  await signIn(page, 'otp', 'https://evil.example/')
  await expect(page).toHaveURL(/localhost:3002\/$/)
  await expect(page.getByRole('heading', { name: 'Disputes' })).toBeVisible()
})
