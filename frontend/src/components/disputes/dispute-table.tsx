import { Link } from '@tanstack/react-router'

import type { components } from '#/api/schema.gen'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { formatMoney } from '#/lib/money'

type Summary = components['schemas']['DisputeSummary']

const None = () => <span className="text-muted-foreground">none</span>

export const DisputeTable = ({ disputes, tenant }: { disputes: Summary[]; tenant: string }) => (
  <div className="overflow-x-auto">
    <table
      className="w-full border-collapse border border-border text-left text-[12px] [&_td]:border-t [&_td]:border-border [&_td]:px-2 [&_td]:py-1.5 [&_th]:border-b-2 [&_th]:border-border [&_th]:px-2 [&_th]:py-1.5 [&_th]:whitespace-nowrap"
      aria-label="Disputes"
    >
      <thead className="bg-[image:var(--panel-heading)] text-[11px] tracking-wide text-muted-foreground uppercase">
        <tr>
          <th className="font-bold">Dispute</th>
          <th className="font-bold">State</th>
          <th className="font-bold">Reason</th>
          <th className="font-bold">Regime</th>
          <th className="font-bold">Amount</th>
          <th className="font-bold">Risk</th>
          <th className="font-bold">Next clock</th>
          <th className="font-bold">Opened</th>
        </tr>
      </thead>
      <tbody className="[&_tr:nth-child(odd)]:bg-muted">
        {disputes.map((dispute) => (
          <tr key={dispute.id} className="hover:bg-accent">
            <td className="font-mono">
              <Link
                to="/$tenant/disputes/$disputeId"
                params={{ tenant, disputeId: dispute.id }}
                className="underline"
              >
                {dispute.id.slice(0, 8)}
              </Link>
            </td>
            <td className="font-mono">{dispute.state}</td>
            <td className="font-mono">{dispute.reason}</td>
            <td className="font-mono">{dispute.regime}</td>
            <td className="whitespace-nowrap">{formatMoney(dispute.disputedAmount, dispute.currency)}</td>
            <td>
              {dispute.riskTier ? <RiskBadge tier={dispute.riskTier} score={dispute.riskScore} /> : <None />}
            </td>
            <td className="whitespace-nowrap">
              {dispute.nextDeadline ? (
                <span className="flex items-center gap-2">
                  <DeadlineBadge status={dispute.nextDeadline.status} />
                  <span className="text-muted-foreground">{daysRemaining(dispute.nextDeadline.dueAt)}</span>
                </span>
              ) : (
                <None />
              )}
            </td>
            <td className="whitespace-nowrap text-muted-foreground">
              {dispute.openedAt.slice(0, 16).replace('T', ' ')}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  </div>
)
