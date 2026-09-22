import { render, screen } from '@testing-library/react'

import { RiskPanel } from './risk'

test('a high score says the credit is on hold and lists every signal', () => {
  render(
    <RiskPanel
      risk={{
        score: 50,
        tier: 'HIGH',
        assessedAt: '2026-09-22T10:00:00Z',
        history: [{ seq: 1, score: 10, tier: 'LOW', assessedAt: '2026-09-21T10:00:00Z' }],
        signals: [
          {
            name: 'dispute frequency',
            weight: 25,
            points: 25,
            detail: '3 other disputes on this account in the last twelve months',
          },
          { name: 'amount', weight: 10, points: 0, detail: 'amount is under 500' },
        ],
      }}
    />,
  )
  expect(screen.getByText(/On hold/)).toBeInTheDocument()
  expect(screen.getByText(/Reassessed 2 times/)).toBeInTheDocument()
  expect(screen.getByRole('table', { name: 'Risk signals' })).toHaveTextContent('25/25')
  expect(screen.getByText(/3 other disputes/)).toBeInTheDocument()
})
