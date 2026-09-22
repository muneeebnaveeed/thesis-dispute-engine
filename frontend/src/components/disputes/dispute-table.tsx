import { Link } from '@tanstack/react-router'

import type { components } from '#/api/schema.gen'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { formatMoney } from '#/lib/money'

type Summary = components['schemas']['DisputeSummary']

const None = () => <span className="text-muted-foreground">none</span>

const COLUMNS = ['Dispute', 'State', 'Reason', 'Regime', 'Amount', 'Risk', 'Next clock', 'Opened'] as const

// the list arrives newest first, and the grid says so the way the period did: a caret on the sorted column
const SORTED = 'Opened'

export const DisputeTable = ({ disputes, tenant }: { disputes: Summary[]; tenant: string }) => (
  <div className="overflow-x-auto">
    <table
      className="w-full border-collapse text-left text-[11px] [&_td]:border-t [&_td]:border-[color:var(--rule)] [&_td]:border-r [&_td]:px-1.5 [&_td]:py-[3px] [&_th]:border-r [&_th]:border-b [&_th]:border-border [&_th]:px-1.5 [&_th]:py-[3px] [&_th]:whitespace-nowrap"
      aria-label="Disputes"
    >
      <thead className="bg-[image:var(--toolbar)] text-foreground">
        <tr>
          {COLUMNS.map((column) => (
            <th
              key={column}
              className="font-bold"
              {...(column === SORTED ? { 'aria-sort': 'descending' as const } : {})}
            >
              {column}
              {column === SORTED && <span className="ml-1 text-[8px] text-primary">▼</span>}
            </th>
          ))}
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
                <span className="flex items-center gap-1.5">
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
