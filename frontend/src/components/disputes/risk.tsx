import type { Dispute } from '#/api/views'
import { Badge, type BadgeTone } from '#/components/ui/badge'
import { cn } from '#/lib/utils'
import { gridBody, gridHead, gridTable } from '#/components/ui/grid'

type Risk = NonNullable<Dispute['risk']>
type RiskTier = Risk['tier']

const RISK_TIER_TONE: Record<RiskTier, BadgeTone> = { LOW: 'good', MEDIUM: 'warn', HIGH: 'bad' }

export const RiskBadge = ({ tier, score }: { tier: RiskTier; score?: number | undefined }) => {
  return (
    <Badge tone={RISK_TIER_TONE[tier]}>
      {tier.toLowerCase()}
      {score !== undefined && <span className="ml-1 opacity-70">{score}</span>}
    </Badge>
  )
}

export const RiskPanel = ({ risk }: { risk: Risk }) => {
  return (
    <div className="space-y-2 text-[11px]">
      <p className="flex items-center gap-2">
        <RiskBadge tier={risk.tier} score={risk.score} />
        <span className="text-muted-foreground">
          {risk.tier === 'HIGH'
            ? 'On hold: a credit needs a recorded justification.'
            : risk.tier === 'MEDIUM'
              ? 'Flagged for review before a credit.'
              : 'Nothing stands out.'}
          {risk.history.length > 0 && ` Reassessed ${risk.history.length + 1} times.`}
        </span>
      </p>
      <table className={gridTable} aria-label="Risk signals">
        <thead className={gridHead}>
          <tr>
            <th>Signal</th>
            <th className="text-right">Points</th>
            <th>Why</th>
          </tr>
        </thead>
        <tbody className={gridBody}>
          {risk.signals.map((signal) => (
            <tr key={signal.name} className={cn(signal.points === 0 && 'text-muted-foreground')}>
              <td>{signal.name}</td>
              <td className="text-right font-mono">
                {signal.points}/{signal.weight}
              </td>
              <td>{signal.detail}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
