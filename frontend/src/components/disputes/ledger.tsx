import { formatMoney } from '#/lib/money'
import { gridBody, gridHead, gridTable } from '#/components/ui/grid'
import type { Dispute } from '#/api/views'
import { cn } from '#/lib/utils'

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
    <div className="space-y-2 text-[11px]">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-1 md:grid-cols-4" aria-label="Balances">
        <Balance label="Customer credited" amount={balances.customer} currency={currency} />
        <Balance label="Advanced, not yet cleared" amount={balances.suspense} currency={currency} emphasise />
        <Balance label="Recovered" amount={balances.recovery} currency={currency} />
        <Balance label="Written off" amount={balances.loss} currency={currency} />
      </dl>
      {ledger.length === 0 ? (
        <p className="text-muted-foreground">No money has moved on this dispute.</p>
      ) : (
        <table className={gridTable} aria-label="Postings">
          <thead className={gridHead}>
            <tr>
              <th>#</th>
              <th>Posting</th>
              <th>Debit</th>
              <th>Credit</th>
              <th className="text-right">Amount</th>
              <th>Core</th>
              <th>Reference</th>
            </tr>
          </thead>
          <tbody className={gridBody}>
            {ledger.map((posting) => (
              <tr key={posting.reference} className="hover:bg-accent">
                <td className="font-mono text-muted-foreground">{posting.seq}</td>
                <td>{POSTING_KIND_LABEL[posting.kind]}</td>
                <td className="font-mono">{posting.debit}</td>
                <td className="font-mono">{posting.credit}</td>
                <td className="text-right font-mono">{formatMoney(posting.amount, posting.currency)}</td>
                <td
                  className="font-mono text-muted-foreground"
                  title={posting.core ? `response ${posting.core.responseCode}` : undefined}
                >
                  {posting.core ? posting.core.rrn || 'booked' : 'internal'}
                </td>
                <td className="font-mono text-muted-foreground">{posting.reference}</td>
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
      <dt className="text-muted-foreground">{label}</dt>
      <dd className={cn('font-mono', stillOutstanding && 'font-medium text-amber-900')}>
        {formatMoney(amount, currency)}
      </dd>
    </div>
  )
}
