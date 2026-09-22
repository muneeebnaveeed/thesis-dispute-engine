import type { Dispute } from '#/api/views'
import { Badge, type BadgeTone } from '#/components/ui/badge'

type Deadline = Dispute['deadlines'][number]

const MILLIS_PER_DAY = 86_400_000

const DEADLINE_STATUS_BADGE: Record<Deadline['status'], { label: string; tone: BadgeTone }> = {
  RUNNING: { label: 'running', tone: 'neutral' },
  MET: { label: 'met', tone: 'good' },
  LATE: { label: 'met late', tone: 'warn' },
  BREACHED: { label: 'breached', tone: 'bad' },
  VOID: { label: 'no longer applies', tone: 'muted' },
}

const DEADLINE_KIND_LABEL: Record<Deadline['kind'], string> = {
  REFUND: 'Make the customer whole',
  ACKNOWLEDGE: 'Acknowledge the dispute',
  RESOLUTION: 'Resolve the dispute',
}

export const DeadlineBadge = ({ status }: { status: Deadline['status'] }) => {
  const badge = DEADLINE_STATUS_BADGE[status]
  return <Badge tone={badge.tone}>{badge.label}</Badge>
}

// whole days: the deadline is an end of day
export const daysRemaining = (dueAt: string, now = Date.now()): string => {
  const millisUntilDue = new Date(dueAt).getTime() - now
  if (millisUntilDue >= 0) {
    const daysLeft = Math.ceil(millisUntilDue / MILLIS_PER_DAY)
    return daysLeft > 1 ? `${daysLeft} days left` : 'due today'
  }
  const daysOver = Math.max(1, Math.ceil(-millisUntilDue / MILLIS_PER_DAY))
  return `${daysOver} day${daysOver === 1 ? '' : 's'} over`
}

export const Deadlines = ({ deadlines }: { deadlines: Deadline[] }) => {
  if (deadlines.length === 0)
    return <p className="text-sm text-neutral-600">This dispute predates the clocks.</p>
  return (
    <ul className="divide-y divide-neutral-200 text-sm" aria-label="Regulatory clocks">
      {deadlines.map((deadline) => {
        const stillCounting = deadline.status === 'RUNNING' || deadline.status === 'BREACHED'
        return (
          <li
            key={`${deadline.kind}-${deadline.cycle}`}
            className="flex flex-wrap items-baseline gap-x-4 gap-y-1 py-2"
          >
            <span className="w-56 font-medium">
              {DEADLINE_KIND_LABEL[deadline.kind]}
              {deadline.cycle > 0 && <span className="ml-1 text-neutral-500">(appeal {deadline.cycle})</span>}
            </span>
            <DeadlineBadge status={deadline.status} />
            <span className="text-neutral-600">
              due {deadline.dueAt.slice(0, 10)}
              {stillCounting && <> ({daysRemaining(deadline.dueAt)})</>}
              {deadline.metAt && <> , met {deadline.metAt.slice(0, 10)}</>}
            </span>
            <span className="basis-full text-xs text-neutral-500">{deadline.basis}</span>
          </li>
        )
      })}
    </ul>
  )
}
