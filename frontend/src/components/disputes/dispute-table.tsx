import { Link } from '@tanstack/react-router'

import type { components } from '#/api/schema.gen'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { formatMoney } from '#/lib/money'

type Summary = components['schemas']['DisputeSummary']

const None = () => <span className="text-neutral-400">none</span>

export const DisputeTable = ({ disputes, tenant }: { disputes: Summary[]; tenant: string }) => (
  <table className="w-full text-left text-sm" aria-label="Disputes">
    <thead className="text-neutral-500">
      <tr>
        <th className="py-1 pr-4 font-normal">Dispute</th>
        <th className="py-1 pr-4 font-normal">State</th>
        <th className="py-1 pr-4 font-normal">Reason</th>
        <th className="py-1 pr-4 font-normal">Regime</th>
        <th className="py-1 pr-4 font-normal">Amount</th>
        <th className="py-1 pr-4 font-normal">Risk</th>
        <th className="py-1 pr-4 font-normal">Next clock</th>
        <th className="py-1 font-normal">Opened</th>
      </tr>
    </thead>
    <tbody>
      {disputes.map((dispute) => (
        <tr key={dispute.id} className="border-t border-neutral-200">
          <td className="py-1 pr-4 font-mono">
            <Link
              to="/$tenant/disputes/$disputeId"
              params={{ tenant, disputeId: dispute.id }}
              className="underline"
            >
              {dispute.id.slice(0, 8)}
            </Link>
          </td>
          <td className="py-1 pr-4 font-mono">{dispute.state}</td>
          <td className="py-1 pr-4 font-mono">{dispute.reason}</td>
          <td className="py-1 pr-4 font-mono">{dispute.regime}</td>
          <td className="py-1 pr-4">{formatMoney(dispute.disputedAmount, dispute.currency)}</td>
          <td className="py-1 pr-4">
            {dispute.riskTier ? <RiskBadge tier={dispute.riskTier} score={dispute.riskScore} /> : <None />}
          </td>
          <td className="py-1 pr-4">
            {dispute.nextDeadline ? (
              <span className="flex items-center gap-2">
                <DeadlineBadge status={dispute.nextDeadline.status} />
                <span className="text-neutral-600">{daysRemaining(dispute.nextDeadline.dueAt)}</span>
              </span>
            ) : (
              <None />
            )}
          </td>
          <td className="py-1 text-neutral-500">{dispute.openedAt.slice(0, 16).replace('T', ' ')}</td>
        </tr>
      ))}
    </tbody>
  </table>
)
