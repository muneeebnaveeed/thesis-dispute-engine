import { render, screen } from '@testing-library/react'

import { EmailComposer } from './email-composer'

test('composer preview markup', () => {
  render(
    <EmailComposer
      templates={[
        {
          kind: 'REQUEST_FOR_INFORMATION',
          label: 'Request for information',
          description: 'Ask for documents.',
          letter: true,
          fields: [
            {
              id: 'items',
              label: 'What we need',
              type: 'MULTISELECT',
              required: true,
              options: [{ key: 'receipt', label: 'Receipt', text: 'a copy of the receipt' }],
            },
            { id: 'days', label: 'Days to respond', type: 'NUMBER', required: true, default: '10' },
          ],
          subject: 'We need a little more information',
          paragraphs: ['We need:', '{{items}}', 'Please reply by {{dueDate}}.'],
        },
      ]}
      facts={{ customer: 'Kovács Anna', bank: 'OTP Bank', today: '2026-09-22' }}
      busy={false}
      onSend={() => Promise.resolve()}
      attachmentDrafts={[{ id: 'a1', filename: 'statement.pdf', size: 2048 }]}
    />,
  )
  expect(screen.getByRole('region', { name: 'Preview' })).toMatchSnapshot()
})
