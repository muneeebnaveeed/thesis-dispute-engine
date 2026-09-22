import { fireEvent, render, screen, within } from '@testing-library/react'
import { vi } from 'vitest'

import { EmailComposer } from './email-composer'

const templates = [
  {
    kind: 'REQUEST_FOR_INFORMATION' as const,
    label: 'Request for information',
    description: 'Ask for documents.',
    letter: true,
    fields: [
      {
        id: 'items',
        label: 'What we need',
        type: 'MULTISELECT' as const,
        required: true,
        options: [{ key: 'receipt', label: 'Receipt', text: 'a copy of the receipt' }],
      },
      { id: 'days', label: 'Days to respond', type: 'NUMBER' as const, required: true, default: '10' },
    ],
    subject: 'We need a little more information',
    paragraphs: ['We need:', '{{items}}', 'Please reply by {{dueDate}}.'],
  },
  {
    kind: 'CUSTOM' as const,
    label: 'Custom message',
    description: 'Your own words.',
    letter: false,
    fields: [
      { id: 'subject', label: 'Subject', type: 'TEXT' as const, required: true },
      { id: 'body', label: 'Message', type: 'TEXTAREA' as const, required: true },
    ],
    subject: '{{subject}}',
    paragraphs: ['{{body}}'],
  },
]
const facts = { customer: 'Kovács Anna', bank: 'OTP Bank', today: '2026-09-22' }

test('the preview fills in as the analyst chooses, and marks what is still missing', () => {
  render(<EmailComposer templates={templates} facts={facts} fields={{}} busy={false} onSend={() => {}} />)
  const preview = screen.getByRole('region', { name: 'Preview' })
  expect(within(preview).getByText('Dear Kovács Anna,')).toBeInTheDocument()
  expect(within(preview).getByText('items')).toHaveAttribute('data-missing', 'items')
  expect(within(preview).getByText(/Please reply by 2 October 2026/)).toBeInTheDocument() // default days
  fireEvent.click(screen.getByLabelText(/Receipt/))
  expect(within(preview).getByText('- a copy of the receipt')).toBeInTheDocument()
  expect(within(preview).queryByText('items')).not.toBeInTheDocument()
})

test('switching template resets the form and sending passes only filled fields', () => {
  const onSend = vi.fn<(template: string, inputs: Record<string, string>) => void>()
  render(<EmailComposer templates={templates} facts={facts} fields={{}} busy={false} onSend={onSend} />)
  fireEvent.change(screen.getByLabelText(/Email template/), { target: { value: 'CUSTOM' } })
  fireEvent.change(screen.getByLabelText(/^Subject/), { target: { value: 'About your card' } })
  fireEvent.change(screen.getByLabelText(/^Message/), { target: { value: 'We have posted a new card.' } })
  const preview = screen.getByRole('region', { name: 'Preview' })
  expect(within(preview).getByRole('heading', { name: 'About your card' })).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Send email' }))
  expect(onSend).toHaveBeenCalledWith('CUSTOM', {
    subject: 'About your card',
    body: 'We have posted a new card.',
  })
})

test('server field errors land under the field they name', () => {
  render(
    <EmailComposer
      templates={templates}
      facts={facts}
      fields={{ 'fields.items': 'choose at least one' }}
      busy={false}
      onSend={() => {}}
    />,
  )
  expect(screen.getByText('choose at least one')).toBeInTheDocument()
})
