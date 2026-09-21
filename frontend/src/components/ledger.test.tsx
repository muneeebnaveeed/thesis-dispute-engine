import { render, screen } from '@testing-library/react'

import { Ledger } from './ledger'

test('shows balances and each posting in plain words', () => {
  render(
    <Ledger
      currency="EUR"
      balances={{ customer: '75.4000', suspense: '75.4000', recovery: '0.0000', loss: '0.0000' }}
      ledger={[
        {
          seq: 3,
          kind: 'FAST_REFUND',
          debit: 'SUSPENSE',
          credit: 'CUSTOMER',
          amount: '75.4000',
          currency: 'EUR',
          reference: 'dispute:x:3:FAST_REFUND',
          postedAt: '2026-09-21T12:00:00Z',
        },
      ]}
    />,
  )
  expect(screen.getByText('Refund to the customer')).toBeInTheDocument()
  expect(screen.getByText('Advanced, not yet cleared').nextSibling).toHaveTextContent('75.4000 EUR')
  expect(screen.getByRole('table', { name: 'Postings' })).toBeInTheDocument()
})

test('an untouched dispute says no money moved', () => {
  render(
    <Ledger
      currency="EUR"
      balances={{ customer: '0.0000', suspense: '0.0000', recovery: '0.0000', loss: '0.0000' }}
      ledger={[]}
    />,
  )
  expect(screen.getByText(/No money has moved/)).toBeInTheDocument()
})
