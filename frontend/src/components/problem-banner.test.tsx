import { render, screen } from '@testing-library/react'

import { ProblemBanner } from './problem-banner'

test('shows the detail, field errors and the retry hint', () => {
  render(
    <ProblemBanner
      problem={{
        type: 'urn:dispute-engine:error:contract-violation',
        title: 'The request is not valid',
        status: 400,
        code: 'contract-violation',
        retryable: false,
        requestId: 'r-1',
        detail: 'the request does not match the API contract',
        errors: [{ field: '/transactionId', message: 'must be a uuid' }],
      }}
    />,
  )
  expect(screen.getByRole('alert')).toHaveTextContent('does not match the API contract')
  expect(screen.getByText('transactionId')).toBeInTheDocument()
  expect(screen.getByText(/will not help/)).toBeInTheDocument()
})
