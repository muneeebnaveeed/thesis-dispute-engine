import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'

import { Value } from '@sinclair/typebox/value'

import { EmailTemplate as EmailTemplateSchema } from '#/api/schemas.gen'
import { renderEmailPreview } from './render'
import type { EmailTemplate } from './render'

// the same inputs as the Go golden tests, so the two halves of the template language can be compared by eye
const templatesDir = join(__dirname, '../../../../backend/internal/dispute/notice/templates')
const templates: EmailTemplate[] = readdirSync(templatesDir)
  .filter((filename) => filename.endsWith('.json'))
  .map((filename) => {
    const parsed: unknown = JSON.parse(readFileSync(join(templatesDir, filename), 'utf8'))
    if (!Value.Check(EmailTemplateSchema, parsed))
      throw new Error(
        `${filename} is not an EmailTemplate: ${JSON.stringify([...Value.Errors(EmailTemplateSchema, parsed)].slice(0, 2))}`,
      )
    return parsed
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

// the server substitutes facts before the browser sees a template
const withFactsSubstituted = (template: EmailTemplate): EmailTemplate => {
  const substituteFacts = (line: string) =>
    line.replace(
      /\{\{(customer|bank|amount|merchant|dispute|today)\}\}/g,
      (match, factName: string) => facts[factName] ?? match,
    )
  return {
    ...template,
    subject: substituteFacts(template.subject),
    paragraphs: template.paragraphs.map(substituteFacts),
  }
}
const today = new Date(`${facts.today}T00:00:00Z`)

test.each(templates.map((template) => [template.kind, template] as const))(
  '%s renders as the analyst previews it',
  (kind, template) => {
    expect(renderEmailPreview(withFactsSubstituted(template), inputs[kind] ?? {}, today)).toMatchSnapshot()
  },
)

test.each(templates.map((template) => [template.kind, template] as const))(
  '%s with nothing filled shows every placeholder',
  (_kind, template) => {
    expect(renderEmailPreview(withFactsSubstituted(template), {}, today)).toMatchSnapshot()
  },
)
