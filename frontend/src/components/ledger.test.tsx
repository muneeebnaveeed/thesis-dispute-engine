import { render, screen } from '@testing-library/react'

import { Ledger } from './ledger'

test('shows balances and each posting in plain words', () => {
  render(
    <Ledger
      currency="EUR"
      balances={{ customer: '75.40', suspense: '75.40', recovery: '0.00', loss: '0.00' }}
      ledger={[
        {
          seq: 3,
          kind: 'FAST_REFUND',
          debit: 'SUSPENSE',
          credit: 'CUSTOMER',
          amount: '75.40',
          currency: 'EUR',
          reference: 'dispute:x:3:FAST_REFUND',
          postedAt: '2026-09-21T12:00:00Z',
        },
      ]}
    />,
  )
  expect(screen.getByText('Refund to the customer')).toBeInTheDocument()
  expect(screen.getByText('Advanced, not yet cleared').nextSibling).toHaveTextContent('EUR 75.40')
  expect(screen.getByRole('table', { name: 'Postings' })).toBeInTheDocument()
})

test('an untouched dispute says no money moved', () => {
  render(
    <Ledger
      currency="EUR"
      balances={{ customer: '0.00', suspense: '0.00', recovery: '0.00', loss: '0.00' }}
      ledger={[]}
    />,
  )
  expect(screen.getByText(/No money has moved/)).toBeInTheDocument()
})
