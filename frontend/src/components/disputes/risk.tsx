import type { Dispute } from '#/api/views'
import { Badge, type BadgeTone } from '#/components/ui/badge'
import { cn } from '#/lib/cn'

type Risk = NonNullable<Dispute['risk']>
type Tier = Risk['tier']

const TONE: Record<Tier, BadgeTone> = { LOW: 'good', MEDIUM: 'warn', HIGH: 'bad' }

/** The tier as a coloured word, with the score; the same on the list and the dispute page. */
export const RiskBadge = ({ tier, score }: { tier: Tier; score?: number | undefined }) => {
  return (
    <Badge tone={TONE[tier]}>
      {tier.toLowerCase()}
      {score !== undefined && <span className="ml-1 opacity-70">{score}</span>}
    </Badge>
  )
}

/** Every signal that was checked, what it added and why, so the score is an argument rather than a number. */
export const RiskPanel = ({ risk }: { risk: Risk }) => {
  return (
    <div className="space-y-3 text-sm">
      <p className="flex items-center gap-2">
        <RiskBadge tier={risk.tier} score={risk.score} />
        <span className="text-neutral-600">
          {risk.tier === 'HIGH'
            ? 'On hold: a credit needs a recorded justification.'
            : risk.tier === 'MEDIUM'
              ? 'Flagged for review before a credit.'
              : 'Nothing stands out.'}
          {risk.history.length > 0 && ` Reassessed ${risk.history.length + 1} times.`}
        </span>
      </p>
      <table className="w-full text-left" aria-label="Risk signals">
        <thead className="text-neutral-500">
          <tr>
            <th className="py-1 pr-4 font-normal">Signal</th>
            <th className="py-1 pr-4 text-right font-normal">Points</th>
            <th className="py-1 font-normal">Why</th>
          </tr>
        </thead>
        <tbody>
          {risk.signals.map((s) => (
            <tr
              key={s.name}
              className={cn('border-t border-neutral-200', s.points === 0 && 'text-neutral-500')}
            >
              <td className="py-1 pr-4">{s.name}</td>
              <td className="py-1 pr-4 text-right font-mono">
                {s.points}/{s.weight}
              </td>
              <td className="py-1">{s.detail}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
