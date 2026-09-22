import { formatMoney } from '#/lib/money'
import type { Dispute } from '#/api/views'
import { cn } from '#/lib/cn'

const POSTING_KIND_LABEL: Record<Dispute['ledger'][number]['kind'], string> = {
  PROVISIONAL_CREDIT: 'Provisional credit to the customer',
  FAST_REFUND: 'Refund to the customer',
  NQA_REFUND: 'Refund to the customer (no questions asked)',
  PROVISIONAL_CREDIT_REVERSAL: 'Provisional credit taken back',
  RECOVERY: 'Recovered',
  WRITE_OFF: 'Written off',
}

export const Ledger = ({ ledger, balances, currency }: Pick<Dispute, 'ledger' | 'balances' | 'currency'>) => {
  return (
    <div className="space-y-3 text-sm">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-1 md:grid-cols-4" aria-label="Balances">
        <Balance label="Customer credited" amount={balances.customer} currency={currency} />
        <Balance label="Advanced, not yet cleared" amount={balances.suspense} currency={currency} emphasise />
        <Balance label="Recovered" amount={balances.recovery} currency={currency} />
        <Balance label="Written off" amount={balances.loss} currency={currency} />
      </dl>
      {ledger.length === 0 ? (
        <p className="text-neutral-600">No money has moved on this dispute.</p>
      ) : (
        <table className="w-full text-left" aria-label="Postings">
          <thead className="text-neutral-500">
            <tr>
              <th className="py-1 pr-4 font-normal">#</th>
              <th className="py-1 pr-4 font-normal">Posting</th>
              <th className="py-1 pr-4 font-normal">Debit</th>
              <th className="py-1 pr-4 font-normal">Credit</th>
              <th className="py-1 pr-4 text-right font-normal">Amount</th>
              <th className="py-1 pr-4 font-normal">Core</th>
              <th className="py-1 font-normal">Reference</th>
            </tr>
          </thead>
          <tbody>
            {ledger.map((posting) => (
              <tr key={posting.reference} className="border-t border-neutral-200">
                <td className="py-1 pr-4 font-mono text-neutral-500">{posting.seq}</td>
                <td className="py-1 pr-4">{POSTING_KIND_LABEL[posting.kind]}</td>
                <td className="py-1 pr-4 font-mono">{posting.debit}</td>
                <td className="py-1 pr-4 font-mono">{posting.credit}</td>
                <td className="py-1 pr-4 text-right font-mono">
                  {formatMoney(posting.amount, posting.currency)}
                </td>
                <td
                  className="py-1 pr-4 font-mono text-xs text-neutral-500"
                  title={posting.core ? `response ${posting.core.responseCode}` : undefined}
                >
                  {posting.core ? posting.core.rrn || 'booked' : 'internal'}
                </td>
                <td className="py-1 font-mono text-xs text-neutral-500">{posting.reference}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

const Balance = ({
  label,
  amount,
  currency,
  emphasise = false,
}: {
  label: string
  amount: string
  currency: string
  emphasise?: boolean
}) => {
  const stillOutstanding = emphasise && Number(amount) > 0
  return (
    <div>
      <dt className="text-neutral-500">{label}</dt>
      <dd className={cn('font-mono', stillOutstanding && 'font-medium text-amber-900')}>
        {formatMoney(amount, currency)}
      </dd>
    </div>
  )
}
