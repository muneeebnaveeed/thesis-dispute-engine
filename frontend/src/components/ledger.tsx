import type { Dispute } from '#/server/disputes'

const KIND: Record<Dispute['ledger'][number]['kind'], string> = {
  PROVISIONAL_CREDIT: 'Provisional credit to the customer',
  FAST_REFUND: 'Refund to the customer',
  NQA_REFUND: 'Refund to the customer (no questions asked)',
  PROVISIONAL_CREDIT_REVERSAL: 'Provisional credit taken back',
  RECOVERY: 'Recovered',
  WRITE_OFF: 'Written off',
}

/** What the engine instructed on this dispute and where the money stands; suspense is what the bank is still out. */
export function Ledger({ ledger, balances, currency }: Pick<Dispute, 'ledger' | 'balances' | 'currency'>) {
  return (
    <div className="space-y-3 text-sm">
      <dl className="grid grid-cols-2 gap-x-6 gap-y-1 md:grid-cols-4" aria-label="Balances">
        <Balance label="Customer credited" value={balances.customer} currency={currency} />
        <Balance label="Advanced, not yet cleared" value={balances.suspense} currency={currency} emphasise />
        <Balance label="Recovered" value={balances.recovery} currency={currency} />
        <Balance label="Written off" value={balances.loss} currency={currency} />
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
              <th className="py-1 font-normal">Reference</th>
            </tr>
          </thead>
          <tbody>
            {ledger.map((e) => (
              <tr key={e.reference} className="border-t border-neutral-200">
                <td className="py-1 pr-4 font-mono text-neutral-500">{e.seq}</td>
                <td className="py-1 pr-4">{KIND[e.kind]}</td>
                <td className="py-1 pr-4 font-mono">{e.debit}</td>
                <td className="py-1 pr-4 font-mono">{e.credit}</td>
                <td className="py-1 pr-4 text-right font-mono">
                  {e.amount} {e.currency}
                </td>
                <td className="py-1 font-mono text-xs text-neutral-500">{e.reference}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}

function Balance({
  label,
  value,
  currency,
  emphasise = false,
}: {
  label: string
  value: string
  currency: string
  emphasise?: boolean
}) {
  const outstanding = emphasise && Number(value) > 0
  return (
    <div>
      <dt className="text-neutral-500">{label}</dt>
      <dd className={`font-mono ${outstanding ? 'font-medium text-amber-900' : ''}`}>
        {value} {currency}
      </dd>
    </div>
  )
}
