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
  await expect(page.getByRole('table', { name: 'Event log' }).getByRole('row')).toHaveCount(3) // header + OPENED + OPEN_INVESTIGATION
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
  await expect(advanced).not.toHaveText(/^0\.0000/)

  // Closing asks how the advance clears; the bank writes it off here and suspense returns to zero.
  const settle = page.getByLabel('Outstanding advance on close')
  await settle.selectOption('WRITTEN_OFF')
  await page.getByRole('button', { name: 'CLOSE' }).click()
  await expect(postings).toContainText('Written off')
  await expect(advanced).toHaveText(/^0\.0000/)
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
  await expect(page.locator('#answer-noticed_on-error')).toContainText(/required/)
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

test('every step writes to the customer: the acknowledgement email lands in the relay and a Reg E letter is ready to print', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}`)
  const comms = page.getByRole('table', { name: 'Communications' })
  const ack = comms.getByRole('row').filter({ hasText: 'Acknowledgement' })
  await expect(ack).toContainText('anna.kovacs@example.com')
  await expect(ack).toContainText(/sent/, { timeout: 20_000 })

  // Mailpit holds what the relay received; the mail names the dispute.
  await expect
    .poll(
      async () => {
        const res = await request.get('http://localhost:8025/api/v1/search', {
          params: { query: `to:anna.kovacs@example.com ${id}` },
        })
        const body = (await res.json()) as { messages_count?: number }
        return body.messages_count ?? 0
      },
      { timeout: 20_000 },
    )
    .toBeGreaterThan(0)

  // Reg E requires written notices: a letter accompanies the email and opens on its own printable page.
  const usd = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${tenants.otp.key}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: '00000000-0000-8000-8000-000000000201', actor: 'e2e' },
  })
  const { id: regE } = (await usd.json()) as { id: string }
  await page.goto(`/otp/disputes/${regE}`)
  const letter = page
    .getByRole('table', { name: 'Communications' })
    .getByRole('row')
    .filter({ hasText: 'LETTER' })
  await expect(letter).toContainText('ready to print')
  await letter.getByRole('link', { name: 'Open letter' }).click()
  await expect(page).toHaveURL(new RegExp(`/otp/disputes/${regE}/notices/\\d+$`))
  await expect(page.getByRole('heading', { name: 'We have received your dispute' })).toBeVisible()
  await expect(page.getByText('Dear Jordan Lee,')).toBeVisible()
  await expect(page.getByText(regE)).toBeVisible()
})

test('a repeat disputer scores HIGH, the credit is held until the analyst records why, and the score is explained', async ({
  page,
  request,
}) => {
  // Three earlier disputes on the same account inside the year, then a large one on a watch-list merchant.
  for (let i = 0; i < 3; i++) await openDisputeViaApi(request, 'otp')
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

test('the communications panel composes an email from a template with a live preview, sends it, and lists it as sent', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}`)
  await page.getByRole('link', { name: 'Open the communications panel' }).click()
  await expect(page).toHaveURL(new RegExp(`/otp/disputes/${id}/communications$`))
  await expect(page.getByRole('tab', { name: /Sent Emails/ })).toContainText('1') // the acknowledgement

  // The preview updates as fields are chosen; unfilled ones are visibly marked.
  await page.getByLabel(/Email template/).selectOption('REQUEST_FOR_INFORMATION')
  const preview = page.getByRole('region', { name: 'Preview' })
  await expect(preview).toContainText('Dear Kovács Anna,')
  await expect(preview.locator('mark[data-missing="items"]')).toBeVisible()
  await page.getByLabel(/Receipt or invoice/).check()
  await page.getByLabel(/Proof of merchant contact/).check()
  await expect(preview).toContainText('- a copy of the receipt')
  await expect(preview).toContainText('- any correspondence with the merchant')
  await expect(preview.locator('mark[data-missing="items"]')).toHaveCount(0)
  await page.getByLabel(/Days to respond/).fill('5')
  await expect(preview).toContainText('(5 days from today)')

  // Sending queues it and switches to Sent Emails, where it is selected and shown as composed.
  await page.getByRole('button', { name: 'Send email' }).click()
  await expect(page).toHaveURL(/tab=sent/)
  const sent = page.getByRole('table', { name: 'Sent emails' })
  const row = sent.getByRole('row').filter({ hasText: 'Request for information' })
  await expect(row).toBeVisible()
  await expect(row).toContainText('analyst')
  await expect(row).toContainText(/sent|queued/)
  const shown = page.getByRole('region', { name: 'Sent email preview' })
  await expect(shown).toContainText('a copy of the receipt')
  await expect(shown).toContainText('5 days from today')

  // A form the server refuses is reported under the field, not as a banner.
  await page.getByRole('tab', { name: 'Create Email' }).click()
  await page.getByLabel(/Email template/).selectOption('CUSTOM')
  await page.getByLabel(/^Subject/).fill('x'.repeat(201))
  await page.getByLabel(/^Message/).fill('Hello')
  await page.getByRole('button', { name: 'Send email' }).click()
  await expect(page.locator('#field-subject-error')).toContainText(/200/)
})

test('a sent email can be resent as a new notice chained to the original', async ({ page, request }) => {
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}/communications?tab=sent`)
  const table = page.getByRole('table', { name: 'Sent emails' })
  const ack = table.getByRole('row').filter({ hasText: 'Acknowledgement' }).first()
  await expect(ack).toContainText(/sent/, { timeout: 20_000 })
  await ack.getByRole('button', { name: 'Resend' }).click()
  await expect(page.getByRole('tab', { name: /Sent Emails/ })).toContainText('2')
  const resent = table.getByRole('row').filter({ hasText: 'Resent: Acknowledgement' })
  await expect(resent).toBeVisible()
  await expect(resent).toContainText('analyst')
  await expect(resent).toContainText(/sent|queued/)
})

test('an email can carry attachments, which the sent view offers back and the relay receives', async ({
  page,
  request,
}) => {
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}/communications`)
  await page.getByLabel(/Email template/).selectOption('CUSTOM')
  await page.getByLabel(/^Subject/).fill('Your statement')
  await page.getByLabel(/^Message/).fill('Please find the statement attached.')

  // A text file is refused under the field; a PDF is accepted and shown as a chip and in the preview.
  await page
    .getByLabel(/Attachments/)
    .setInputFiles({ name: 'notes.txt', mimeType: 'text/plain', buffer: Buffer.from('hi') })
  await expect(page.locator('#field-attachments-error')).toContainText(/PDF, PNG or JPEG/)
  await page.getByLabel(/Attachments/).setInputFiles({
    name: 'statement.pdf',
    mimeType: 'application/pdf',
    buffer: Buffer.from('%PDF-1.4 e2e'),
  })
  const chips = page.getByRole('list', { name: 'Attached files' })
  await expect(chips).toContainText('statement.pdf')
  await expect(page.getByRole('region', { name: 'Preview' })).toContainText('Attached: statement.pdf')

  await page.getByRole('button', { name: 'Send email' }).click()
  await expect(page).toHaveURL(/tab=sent/)
  const shown = page.getByRole('region', { name: 'Sent email preview' })
  await expect(shown.getByRole('list', { name: 'Attachments' })).toContainText('statement.pdf')

  // The relay got the file: Mailpit reports one attachment on the message.
  await expect
    .poll(
      async () => {
        const res = await request.get('http://localhost:8025/api/v1/search', {
          params: { query: `subject:"Your statement" ${id}` },
        })
        const body = (await res.json()) as { messages?: { Attachments?: number }[] }
        return body.messages?.[0]?.Attachments ?? 0
      },
      { timeout: 20_000 },
    )
    .toBe(1)
})
