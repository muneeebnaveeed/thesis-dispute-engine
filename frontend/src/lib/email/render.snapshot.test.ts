import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

import { Value } from '@sinclair/typebox/value'

import { EmailTemplate as EmailTemplateSchema } from '#/api/schemas.gen'
import { preview } from './render'
import type { EmailTemplate } from './render'

// The browser-side renderer against the real template files: the same inputs as the Go golden tests, so the two
// halves of the template language can be compared by eye. Update with: pnpm vitest run -u
const dir = join(__dirname, '../../../../backend/internal/dispute/notice/templates')
const templates: EmailTemplate[] = readdirSync(dir)
  .filter((f) => f.endsWith('.json'))
  .map((f) => {
    const raw: unknown = JSON.parse(readFileSync(join(dir, f), 'utf8'))
    if (!Value.Check(EmailTemplateSchema, raw))
      throw new Error(
        `${f} is not an EmailTemplate: ${JSON.stringify([...Value.Errors(EmailTemplateSchema, raw)].slice(0, 2))}`,
      )
    return raw
  })

const facts: Record<string, string> = {
  customer: 'Kovács Anna',
  bank: 'OTP Bank',
  amount: '1899.00 EUR',
  merchant: 'MediaMarkt',
  dispute: '01a0c601-0000-7000-8000-000000000001',
  today: '2026-09-22',
}
const inputs: Record<string, Record<string, string>> = {
  REQUEST_FOR_INFORMATION: {
    items: 'receipt,merchant_contact',
    other: 'the serial number\nthe box it came in',
    days: '10',
  },
  STATUS_UPDATE: { stage: 'merchant', note: 'The merchant has 30 days to answer.', nextBy: '2026-10-20' },
  DOCUMENTS_RECEIVED: {
    received: 'receipt dated 9 September\nphotos of the item',
    missing: "the courier's proof of delivery",
  },
  CUSTOM: {
    subject: 'Your new card',
    body: 'We have posted a replacement card to your address on file.\nIt arrives within five working days.',
  },
}

// The server substitutes facts before the browser sees a template; do the same here.
const withFacts = (t: EmailTemplate): EmailTemplate => {
  const sub = (s: string) =>
    s.replace(/\{\{(customer|bank|amount|merchant|dispute|today)\}\}/g, (m, k: string) => facts[k] ?? m)
  return { ...t, subject: sub(t.subject), paragraphs: t.paragraphs.map(sub) }
}

test.each(templates.map((t) => [t.kind, t] as const))('%s renders as the analyst previews it', (kind, t) => {
  expect(preview(withFacts(t), inputs[kind] ?? {}, new Date(`${facts.today}T00:00:00Z`))).toMatchSnapshot()
})

test.each(templates.map((t) => [t.kind, t] as const))(
  '%s with nothing filled shows every placeholder',
  (_kind, t) => {
    expect(preview(withFacts(t), {}, new Date(`${facts.today}T00:00:00Z`))).toMatchSnapshot()
  },
)
