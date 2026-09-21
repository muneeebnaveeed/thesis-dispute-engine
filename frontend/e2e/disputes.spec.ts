import { expect, openDisputeViaApi, signIn, tenants, test, api } from './fixtures'

test.beforeEach(async ({ page }) => {
  await signIn(page, 'otp')
})

test('an analyst opens a dispute and drives it through allowed transitions from the browser', async ({
  page,
}) => {
  await page.getByLabel('Transaction ID').fill(tenants.otp.transaction)
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page).toHaveURL(/\/disputes\/[0-9a-f-]{36}$/)
  const state = page
    .getByRole('definition')
    .filter({ hasText: /^[A-Z_]+$/ })
    .first()
  await expect(state).toHaveText('INITIATED')
  await expect(page.getByRole('definition').filter({ hasText: 'EU_PSD2_CARD' })).toBeVisible()

  // Actions go straight from the browser to the API with a bearer token the server handed out.
  const direct: string[] = []
  page.on('request', (r) => {
    if (r.url().startsWith(api) && r.method() === 'POST') direct.push(r.headers().authorization ?? '')
  })
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  await expect(state).toHaveText('INVESTIGATING')
  await expect(page.getByRole('row')).toHaveCount(3) // header + OPENED + OPEN_INVESTIGATION
  expect(direct.length).toBeGreaterThan(0)
  expect(direct[0]).toMatch(/^Bearer ey/)
  expect(direct[0]).not.toContain('tk_')

  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(state).toHaveText('FAST_REFUND_ISSUED')
  await expect(page.getByRole('button', { name: 'CLOSE' })).toBeVisible()
})

test('structural mistakes are caught before the API and shown under the field', async ({ page }) => {
  await page.getByLabel('Transaction ID').fill('not-a-uuid')
  await page.getByRole('button', { name: 'Open' }).click()
  const alert = page.getByRole('alert')
  await expect(alert).toContainText('not valid')
  await expect(alert).toContainText('transactionId')
  await expect(alert).toContainText('will not help')
})

test("another tenant's data does not exist for this analyst", async ({ page, request }) => {
  const foreign = await openDisputeViaApi(request, 'erste')
  await page.goto(`/disputes/${foreign}`)
  await expect(page.getByRole('alert')).toContainText('does not exist')

  await page.goto('/')
  await page.getByLabel('Transaction ID').fill(tenants.erste.transaction)
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page.getByRole('alert')).toContainText('does not exist')
})
