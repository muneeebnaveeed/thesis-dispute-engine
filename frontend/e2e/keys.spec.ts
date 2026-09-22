import { api, expect, openDisputeViaApi, signIn, test } from './fixtures'

test('a tenant admin issues a key, the key works for that tenant, and revoking it stops it', async ({
  page,
  request,
}) => {
  await signIn(page, 'otp')
  await page.getByRole('link', { name: 'Keys' }).click()
  await expect(page).toHaveURL(/\/otp\/keys$/)

  const label = `e2e ${Date.now()}`
  await page.getByLabel('Label').fill(label)
  await page.getByRole('button', { name: 'Issue' }).click()
  const shown = page.getByRole('status')
  await expect(shown).toContainText(label)
  const secret = (await shown.locator('code').textContent())?.trim() ?? ''
  expect(secret).toMatch(/^tk_/)

  // The secret is a machine credential for OTP Bank: it works there and sees nothing of Erste.
  const mine = await request.get(`${api}/disputes/00000000-0000-8000-8000-000000000000`, {
    headers: { Authorization: `Bearer ${secret}` },
  })
  expect(mine.status()).toBe(404)
  const created = await request.post(`${api}/disputes`, {
    headers: { Authorization: `Bearer ${secret}`, 'Idempotency-Key': crypto.randomUUID() },
    data: { transactionId: '00000000-0000-8000-8000-000000000301', actor: 'e2e' },
  })
  expect(created.status()).toBe(404)

  // The list shows it by prefix only, live; revoking makes the next use a 401.
  const row = page.getByRole('row').filter({ hasText: label })
  await expect(row).toContainText(secret.slice(0, 12))
  await expect(row).toContainText('live')
  await expect(page.getByText(secret.slice(12))).toHaveCount(1) // only in the one-time box, never in the table
  await row.getByRole('button', { name: 'Revoke' }).click()
  await expect(row).toContainText('revoked')
  const after = await request.get(`${api}/disputes/00000000-0000-8000-8000-000000000000`, {
    headers: { Authorization: `Bearer ${secret}` },
  })
  expect(after.status()).toBe(401)
})

test('an analyst without the admin role cannot manage keys', async ({ request }) => {
  // The seeded analysts are admins; the API-level rule is covered by the contract tests, so here we only assert a
  // tenant key (a machine credential) is refused with forbidden, not unauthenticated.
  const res = await request.get(`${api}/tenant-keys`, {
    headers: { Authorization: 'Bearer tk_dev_tenant_a' },
  })
  expect(res.status()).toBe(403)
  expect(((await res.json()) as { code: string }).code).toBe('forbidden')
})

test('a tenant admin rewords an email template and analysts compose with the new words', async ({
  page,
  request,
}) => {
  await signIn(page, 'otp')
  await page.getByRole('link', { name: 'Templates' }).click()
  await expect(page).toHaveURL(/\/otp\/templates$/)
  const card = page.getByRole('region', { name: 'Status update', exact: true })
  await card.getByRole('button', { name: 'Edit wording' }).click()
  await card.getByLabel('Subject').fill('Where your {{merchant}} dispute stands')
  // An unknown placeholder is flagged in the preview and refused on save.
  await card.getByLabel(/Paragraphs/).fill('{{stage}}\n\n{{nothing}}\n\nReference {{dispute}}.')
  await expect(card.getByRole('region', { name: /Preview of Status update/ })).toContainText(
    'unknown: nothing',
  )
  await card.getByRole('button', { name: 'Save wording' }).click()
  await expect(card.locator('#template-STATUS_UPDATE-error')).toBeVisible()
  await card.getByLabel(/Paragraphs/).fill('{{stage}}\n\n{{note}}\n\nReference {{dispute}}. Yours, {{bank}}.')
  await card.getByRole('button', { name: 'Save wording' }).click()
  await expect(page.getByRole('status')).toContainText(/wording saved/)
  await expect(card).toContainText('customised')

  // The analyst's composer now shows the tenant's subject with the merchant filled in.
  const id = await openDisputeViaApi(request, 'otp')
  await page.goto(`/otp/disputes/${id}/communications`)
  await page.getByLabel(/Email template/).selectOption('STATUS_UPDATE')
  await expect(page.getByRole('region', { name: 'Preview' })).toContainText(
    'Where your MediaMarkt dispute stands',
  )

  // Revert restores the standard wording.
  await page.goto('/otp/templates')
  await card.getByRole('button', { name: 'Edit wording' }).click()
  await card.getByRole('button', { name: 'Revert to standard' }).click()
  await expect(page.getByRole('status')).toContainText(/Reverted/)
})
