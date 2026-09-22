import { Link } from '@tanstack/react-router'

import type { components } from '#/api/schema.gen'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { gridBody, gridHead, gridTable } from '#/components/ui/grid'
import { formatMoney } from '#/lib/money'

type Summary = components['schemas']['DisputeSummary']

const None = () => <span className="text-muted-foreground">none</span>

const COLUMNS = ['Dispute', 'State', 'Reason', 'Regime', 'Amount', 'Risk', 'Next clock', 'Opened'] as const

// the list arrives newest first, and the grid says so the way the period did: a caret on the sorted column
const SORTED = 'Opened'

export const DisputeTable = ({ disputes, tenant }: { disputes: Summary[]; tenant: string }) => (
  <div className="overflow-x-auto">
    <table className={gridTable} aria-label="Disputes">
      <thead className={gridHead}>
        <tr>
          {COLUMNS.map((column) => (
            <th key={column} {...(column === SORTED ? { 'aria-sort': 'descending' as const } : {})}>
              {column}
              {column === SORTED && <span className="ml-1 text-[8px] text-primary">▼</span>}
            </th>
          ))}
        </tr>
      </thead>
      <tbody className={gridBody}>
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
