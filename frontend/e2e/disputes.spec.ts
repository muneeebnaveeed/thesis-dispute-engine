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

  // Everything goes through the app server (ADR 0020): the browser never talks to the API and never holds a token.
  const requestsToApi: string[] = []
  page.on('request', (request) => {
    if (request.url().startsWith(api)) requestsToApi.push(`${request.method()} ${request.url()}`)
  })
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  await expect(state).toHaveText('INVESTIGATING')
  await expect(page.getByRole('table', { name: 'Event log' }).getByRole('row')).toHaveCount(3) // header + OPENED + OPEN_INVESTIGATION
  expect(requestsToApi).toEqual([])

  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(state).toHaveText('FAST_REFUND_ISSUED')
  await expect(page.getByRole('button', { name: 'CLOSE' })).toBeVisible()
})

test('structural mistakes are caught in the browser and shown under the field, with no request made', async ({
  page,
}) => {
  const requestsToApi: string[] = []
  page.on('request', (request) => {
    if (request.url().startsWith(api)) requestsToApi.push(request.url())
  })
  const input = page.getByLabel('Transaction ID')
  await input.fill('not-a-uuid')
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(input).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('#transactionId-error')).toBeVisible()
  await expect(page.getByRole('alert')).toHaveCount(0)
  expect(requestsToApi.filter((url) => url.includes('/disputes')).length).toBe(0)
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

test('a refund with a customer liability posts the difference, an over-cap liability is refused under the field, and closing clears suspense', async ({
  page,
}) => {
  const id = await openDisputeViaApi(page.request, 'otp')
  await page.goto(`/otp/disputes/${id}`)
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  const liability = page.getByLabel(/Customer liability/)
  await expect(liability).toBeVisible()

  await liability.fill('50.01') // PSD2 art. 74 caps the customer's share at 50
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(liability).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('#liability-error')).toContainText(/cap/)
  await expect(page.getByText(/No money has moved/)).toBeVisible()

  await liability.fill('50')
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  const postings = page.getByRole('table', { name: 'Postings' })
  await expect(postings).toContainText('Refund to the customer')
  await expect(postings).toContainText('SUSPENSE')
  const advanced = page.getByText('Advanced, not yet cleared').locator('xpath=following-sibling::dd')
  await expect(advanced).not.toHaveText(/EUR 0\.00$/)

  // Closing asks how the advance clears; the bank writes it off here and suspense returns to zero.
  const settle = page.getByLabel('Outstanding advance on close')
  await settle.selectOption('WRITTEN_OFF')
  await page.getByRole('button', { name: 'CLOSE' }).click()
  await expect(postings).toContainText('Written off')
  await expect(advanced).toHaveText(/EUR 0\.00$/)
})

test("the tenant's banking core answers each credit, and a decline leaves the dispute exactly as it was", async ({
  page,
  request,
}) => {
  // OTP's simulated core refuses credits above 5000 EUR; the seeded 7450 EUR Apple Store purchase trips it.
  const res = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${tenants.otp.key}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: '00000000-0000-8000-8000-000000000104', actor: 'e2e' },
  })
  expect(res.status()).toBe(201)
  const { id } = (await res.json()) as { id: string }
  await page.goto(`/otp/disputes/${id}`)
  // On a well-used account the fraud score may already hold the credit; that gate is exercised elsewhere.
  const onHold = await page.getByText(/On hold/).isVisible()
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  const why = page.getByLabel(/Justification for crediting/)
  if (onHold) await why.fill('e2e: exercising the core decline')
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  const alert = page.getByRole('alert')
  await expect(alert).toContainText(/declined/)
  await expect(alert).toContainText('61')
  await expect(
    page
      .getByRole('definition')
      .filter({ hasText: /^[A-Z_]+$/ })
      .first(),
  ).toHaveText('INVESTIGATING')
  await expect(page.getByText(/No money has moved/)).toBeVisible()

  // A smaller dispute is credited and the ledger shows the core's retrieval reference.
  const small = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${small}`)
  const smallOnHold = await page.getByText(/On hold/).isVisible()
  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  if (smallOnHold) await why.fill('e2e: exercising the core receipt')
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  const row = page
    .getByRole('table', { name: 'Postings' })
    .getByRole('row')
    .filter({ hasText: 'Refund to the customer' })
  await expect(row).toContainText(/[0-9A-F]{12}/)
})

test('the questionnaire follows the reason, incomplete answers are refused under each question, and contradictions are flagged', async ({
  page,
}) => {
  await page.getByLabel('Transaction ID').fill(tenants.otp.transaction)
  await page.getByLabel('Reason').selectOption('UNAUTHORISED')
  await page.getByRole('button', { name: 'Open' }).click()
  await expect(page).toHaveURL(/\/otp\/disputes\/[0-9a-f-]{36}$/)
  await expect(page.getByRole('definition').filter({ hasText: 'UNAUTHORISED' })).toBeVisible()

  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  await page.getByRole('button', { name: 'SEND_QUESTIONNAIRE' }).click()
  const merchant = page.getByLabel(/Do you recognise the merchant/)
  await expect(merchant).toBeVisible()

  // Only one answer given: every required question is called out, and the state has not moved.
  await merchant.selectOption('yes')
  await page.getByRole('button', { name: 'Record answers' }).click()
  const noticed = page.getByLabel(/When did you notice/)
  await expect(noticed).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('#answers-noticed_on-error')).toContainText(/Please select a date/)
  await expect(
    page
      .getByRole('definition')
      .filter({ hasText: /^[A-Z_]+$/ })
      .first(),
  ).toHaveText('QUESTIONNAIRE_SENT')

  await page.getByLabel(/card in your possession/).selectOption('yes')
  await page.getByLabel(/anyone else had access/).selectOption('no')
  await page.getByLabel(/disputed a transaction with this merchant before/).selectOption('no')
  await noticed.fill('2026-09-20')
  await page.getByLabel(/lost or stolen/).selectOption('no')
  await page.getByRole('button', { name: 'Record answers' }).click()
  await expect(
    page
      .getByRole('definition')
      .filter({ hasText: /^[A-Z_]+$/ })
      .first(),
  ).toHaveText('QUESTIONNAIRE_RECEIVED')
  await expect(page.getByRole('list', { name: 'Inconsistencies' })).toContainText(/recognises the merchant/)
})

test('a repeat disputer scores HIGH, the credit is held until the analyst records why, and the score is explained', async ({
  page,
  request,
}) => {
  // Three earlier disputes on the same account inside the year, then a large one on a watch-list merchant.
  for (let earlier = 0; earlier < 3; earlier++) await openDisputeViaApi(request, 'otp')
  const res = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${tenants.otp.key}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: '00000000-0000-8000-8000-000000000104', actor: 'e2e' }, // 7450 EUR, MCC 5815
  })
  const { id } = (await res.json()) as { id: string }
  await page.goto(`/otp/disputes/${id}`)
  const signals = page.getByRole('table', { name: 'Risk signals' })
  await expect(signals).toContainText('other disputes on this account')
  await expect(page.getByText(/On hold: a credit needs a recorded justification/)).toBeVisible()

  await page.getByRole('button', { name: 'OPEN_INVESTIGATION' }).click()
  const why = page.getByLabel(/Justification for crediting/)
  await expect(why).toBeVisible()
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(why).toHaveAttribute('aria-invalid', 'true')
  await expect(page.locator('#riskOverride-error')).toContainText(/HIGH/)
  await expect(
    page
      .getByRole('definition')
      .filter({ hasText: /^[A-Z_]+$/ })
      .first(),
  ).toHaveText('INVESTIGATING')

  // With a justification the hold lifts; this tenant's core then declines the amount, which is the next gate.
  await why.fill('Customer verified in branch; police report 2026/1234 confirms the card was stolen.')
  await page.getByRole('button', { name: 'ISSUE_REFUND' }).click()
  await expect(page.getByRole('alert')).toContainText(/declined/)
})
