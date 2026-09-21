import { expect, openDisputeViaApi, signIn, tenants, test, api } from './fixtures'

test.beforeEach(async ({ page }) => {
  await signIn(page, 'otp')
})

test('an analyst opens a dispute and drives it through allowed transitions from the browser', async ({
  page,
}) => {
  await page.getByLabel('Transaction ID').fill(tenants.otp.transaction)
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page).toHaveURL(/\/otp\/disputes\/[0-9a-f-]{36}$/)
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
  await page.goto(`/otp/disputes/${foreign}`)
  await expect(page.getByRole('alert')).toContainText('does not exist')

  await page.goto('/otp')
  await page.getByLabel('Transaction ID').fill(tenants.erste.transaction)
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page.getByRole('alert')).toContainText('does not exist')
})

test("the workbench lists the tenant's newest disputes, filters by state, and links to each", async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await openDisputeViaApi(request, 'erste')
  await page.goto('/otp')
  // Ids minted in the same millisecond share a visible prefix; the link's href carries the whole id.
  const row = page.getByRole('row').filter({ has: page.locator(`a[href$="/disputes/${id}"]`) })
  await expect(row).toBeVisible()
  await expect(row).toContainText('INITIATED')

  // Filtering to a state this dispute is not in hides it; the filter value comes from the contract's enum.
  await page.getByLabel('State').selectOption('CLOSED')
  await page.getByRole('button', { name: 'Filter' }).click()
  await expect(page).toHaveURL(/state=CLOSED/)
  await expect(row).toHaveCount(0)
  await page.getByLabel('State').selectOption('INITIATED')
  await page.getByRole('button', { name: 'Filter' }).click()
  await expect(row).toBeVisible()

  await row.getByRole('link').click()
  await expect(page).toHaveURL(new RegExp(`/otp/disputes/${id}$`))
})
