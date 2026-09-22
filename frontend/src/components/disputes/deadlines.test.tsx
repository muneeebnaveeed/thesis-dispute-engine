import { render, screen } from '@testing-library/react'

import { Deadlines, remaining } from './deadlines'

const day = 86_400_000
const now = Date.UTC(2026, 8, 21, 12)

test('remaining speaks in whole days either side of due', () => {
  expect(remaining(new Date(now + 3 * day).toISOString(), now)).toBe('3 days left')
  expect(remaining(new Date(now + 6 * 3_600_000).toISOString(), now)).toBe('due today')
  expect(remaining(new Date(now - 2 * 3_600_000).toISOString(), now)).toBe('1 day over')
  expect(remaining(new Date(now - 1 * day).toISOString(), now)).toBe('1 day over')
  expect(remaining(new Date(now - 5 * day).toISOString(), now)).toBe('5 days over')
})

test('each clock shows its purpose, status and legal basis', () => {
  render(
    <Deadlines
      deadlines={[
        {
          kind: 'REFUND',
          cycle: 0,
          startedAt: '2026-09-21T12:00:00Z',
          dueAt: '2026-09-22T23:59:59Z',
          metAt: '2026-09-21T13:00:00Z',
          status: 'MET',
          basis: 'PSD2 art. 73(1)',
        },
        {
          kind: 'RESOLUTION',
          cycle: 1,
          startedAt: '2026-09-21T12:00:00Z',
          dueAt: '2026-10-12T23:59:59Z',
          status: 'BREACHED',
          basis: 'PSD2 art. 101(2)',
        },
      ]}
    />,
  )
  expect(screen.getByText('Make the customer whole')).toBeInTheDocument()
  expect(screen.getByText('met')).toBeInTheDocument()
  expect(screen.getByText('(appeal 1)')).toBeInTheDocument()
  expect(screen.getByText('breached')).toBeInTheDocument()
  expect(screen.getByText('PSD2 art. 101(2)')).toBeInTheDocument()
})

test('a dispute without clocks says so instead of showing nothing', () => {
  render(<Deadlines deadlines={[]} />)
  expect(screen.getByText(/predates the clocks/)).toBeInTheDocument()
})
