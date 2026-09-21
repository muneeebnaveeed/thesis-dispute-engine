import { act, render, screen } from '@testing-library/react'
import { vi } from 'vitest'

import { fromProblem, type Problem } from '#/api/failure'
import { FailureBanner, FieldError } from './failure-banner'

const base: Problem = {
  type: 'urn:x',
  title: 't',
  status: 429,
  code: 'rate-limited',
  retryable: true,
  requestId: 'r-9',
  retryAfterSeconds: 2,
}

test('a rate limit shows the wait and unlocks a retry when it passes', () => {
  vi.useFakeTimers()
  const onRetry = vi.fn<() => void>()
  render(<FailureBanner failure={fromProblem(base)} onRetry={onRetry} />)
  expect(screen.getByRole('alert')).toHaveTextContent('Too many requests')
  expect(screen.getByText(/Retry available in 2s/)).toBeInTheDocument()
  act(() => {
    vi.advanceTimersByTime(2100)
  })
  expect(screen.getByRole('button', { name: 'Retry now' })).toBeInTheDocument()
  expect(screen.getByText(/Reference r-9/)).toBeInTheDocument()
  vi.useRealTimers()
})

test('a conflict lists what is allowed now and a local validation shows no reference', () => {
  render(
    <FailureBanner
      failure={fromProblem({
        ...base,
        status: 409,
        code: 'invalid-transition',
        retryable: false,
        allowedEvents: ['CLOSE'],
      })}
    />,
  )
  expect(screen.getByRole('alert')).toHaveTextContent('Allowed now: CLOSE')
  render(<FieldError id="f" message="must be a uuid" />)
  expect(screen.getByText('must be a uuid')).toHaveAttribute('id', 'f')
})
