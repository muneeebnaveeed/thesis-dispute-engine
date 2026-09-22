import { displayValues, preview, segments } from './render'

const today = new Date('2026-09-22T12:00:00Z')
const rfi = {
  kind: 'REQUEST_FOR_INFORMATION' as const,
  label: 'Request for information',
  description: '',
  letter: true,
  fields: [
    {
      id: 'items',
      label: 'What we need',
      type: 'MULTISELECT' as const,
      required: true,
      options: [
        { key: 'receipt', label: 'Receipt', text: 'a copy of the receipt' },
        { key: 'delivery', label: 'Delivery', text: 'the expected delivery date' },
      ],
    },
    { id: 'other', label: 'Other', type: 'TEXTAREA' as const, required: false },
    { id: 'days', label: 'Days', type: 'NUMBER' as const, required: true, default: '10' },
    { id: 'nextBy', label: 'Next', type: 'DATE' as const, required: false },
  ],
  subject: 'We need more about your dispute',
  paragraphs: [
    'We are looking into the 1899.00 EUR payment to MediaMarkt. We need:',
    '{{items}}',
    '{{other}}',
    'Please reply by {{dueDate}} ({{days}} days from today).',
    '{{#nextBy}}We will write again by {{nextBy}}.{{/nextBy}}',
  ],
}

test('display values turn keys into customer text and compute the due date', () => {
  const v = displayValues(rfi.fields, { items: 'receipt,delivery', nextBy: '2026-10-01' }, today)
  expect(v.items).toBe('a copy of the receipt\nthe expected delivery date')
  expect(v.days).toBe('10')
  expect(v.dueDate).toBe('2 October 2026')
  expect(v.nextBy).toBe('1 October 2026')
})

test('preview renders lists as bullets, drops empty paragraphs and resolves sections', () => {
  const p = preview(rfi, { items: 'receipt', other: 'the serial number\n' }, today)
  expect(p.paragraphs).toEqual([
    'We are looking into the 1899.00 EUR payment to MediaMarkt. We need:',
    '- a copy of the receipt',
    '- the serial number',
    'Please reply by 2 October 2026 (10 days from today).',
  ])
  const withDate = preview(rfi, { items: 'receipt', nextBy: '2026-10-01' }, today)
  expect(withDate.paragraphs.at(-1)).toBe('We will write again by 1 October 2026.')
})

test('unfilled placeholders stay visible and are marked as missing', () => {
  const p = preview(rfi, {}, today)
  expect(p.paragraphs[1]).toBe('{{items}}')
  expect(segments('Please reply by {{dueDate}} soon.')).toEqual([
    { text: 'Please reply by ', missing: false },
    { text: 'dueDate', missing: true },
    { text: ' soon.', missing: false },
  ])
})
