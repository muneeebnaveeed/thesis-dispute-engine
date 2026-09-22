import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { vi } from 'vitest'

import type { Failure, Problem } from '#/api/failure'
import { QuestionnairePanel } from './questionnaire'

const sent = {
  reason: 'DUPLICATE' as const,
  questions: [
    {
      id: 'original_on',
      text: 'When was the transaction you did authorise?',
      type: 'DATE' as const,
      required: true,
    },
    { id: 'same_merchant', text: 'Was it with the same merchant?', type: 'YES_NO' as const, required: true },
    { id: 'details', text: 'Anything else we should know?', type: 'TEXT' as const, required: false },
  ],
  inconsistencies: [],
  sentAt: '2026-09-22T10:00:00Z',
}

test('an unanswered questionnaire is a form that sends only what was filled in', async () => {
  const onReceive = vi
    .fn<(answers: Record<string, string>) => Promise<unknown>>()
    .mockResolvedValue(undefined)
  render(<QuestionnairePanel questionnaire={sent} canReceive busy={false} onReceive={onReceive} />)
  fireEvent.change(screen.getByLabelText(/same merchant/), { target: { value: 'no' } })
  fireEvent.change(screen.getByLabelText(/did authorise/), { target: { value: '2026-09-10' } })
  fireEvent.click(screen.getByRole('button', { name: 'Record answers' }))
  await waitFor(() =>
    expect(onReceive).toHaveBeenCalledWith({ original_on: '2026-09-10', same_merchant: 'no' }),
  )
})

test('server field errors land under the question they are about', async () => {
  const problem: Problem = {
    type: 'urn:dispute-engine:error:invalid-answers',
    title: 'invalid answers',
    status: 422,
    code: 'invalid-answers',
    retryable: false,
    requestId: 'r1',
  }
  const refused: Failure = {
    kind: 'validation',
    problem,
    fields: { 'answers.original_on': 'must be a date as YYYY-MM-DD' },
  }
  render(
    <QuestionnairePanel
      questionnaire={sent}
      canReceive
      busy={false}
      onReceive={() => Promise.reject(refused)}
    />,
  )
  fireEvent.click(screen.getByRole('button', { name: 'Record answers' }))
  await waitFor(() => expect(screen.getByLabelText(/did authorise/)).toHaveAttribute('aria-invalid', 'true'))
  expect(screen.getByText('must be a date as YYYY-MM-DD')).toBeInTheDocument()
})

test('a received questionnaire shows the answers and any contradictions', () => {
  render(
    <QuestionnairePanel
      questionnaire={{
        ...sent,
        answers: { original_on: '2026-09-10', same_merchant: 'no' },
        receivedAt: '2026-09-23T09:00:00Z',
        inconsistencies: ['a duplicate is claimed against a different merchant'],
      }}
      canReceive={false}
      busy={false}
      onReceive={() => Promise.resolve()}
    />,
  )
  expect(screen.getByRole('list', { name: 'Inconsistencies' })).toHaveTextContent(/different merchant/)
  expect(screen.getByText('not answered')).toBeInTheDocument()
  expect(screen.queryByRole('button')).not.toBeInTheDocument()
})
