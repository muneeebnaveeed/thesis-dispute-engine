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

test('structural mistakes are caught in the browser and shown under the field, with no request made', async ({
  page,
}) => {
  const requests: string[] = []
  page.on('request', (r) => {
    if (r.url().startsWith(api)) requests.push(r.url())
  })
  const input = page.getByLabel('Transaction ID')
  await input.fill('not-a-uuid')
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(input).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('#transactionId-error')).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
  expect(requests.filter((u) => u.includes('/disputes')).length).toBe(0)
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

test('the regime clocks appear at opening, settle with the transition that satisfies them, and show on the list', async ({
  page,
}) => {
  const id = await openDisputeViaApi(page.request, 'otp')
  await page.goto(`/otp/disputes/${id}`)
  const clocks = page.getByRole('list', { name: 'Regulatory clocks' })
  const refund = clocks.getByRole('listitem').filter({ hasText: 'Make the customer whole' })
  const resolution = clocks.getByRole('listitem').filter({ hasText: 'Resolve the dispute' })
  await expect(refund).toContainText('running')
  await expect(refund).toContainText('PSD2 art. 73(1)')
  await expect(resolution).toContainText('running')

  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(refund).toContainText('met')
  await expect(refund).toContainText('met 20')
  await expect(resolution).toContainText('running')

  // The list carries the clock that runs out next; the overdue filter has nothing for a fresh dispute.
  await page.goto('/otp')
  const row = page.getByRole('row').filter({ has: page.locator(`a[href$="/${id}"]`) })
  await expect(row).toContainText('running')
  await expect(row).toContainText(/days left|due today/)
  await page.getByLabel('overdue only').check()
  await page.getByRole('button', { name: 'Filter' }).click()
  await expect(page).toHaveURL(/overdue=true/)
  await expect(row).toHaveCount(0)
})
