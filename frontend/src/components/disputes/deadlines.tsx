import type { Dispute } from '#/api/views'
import { Badge, type BadgeTone } from '#/components/ui/badge'

type Deadline = Dispute['deadlines'][number]

const STATUS: Record<Deadline['status'], { label: string; tone: BadgeTone }> = {
  RUNNING: { label: 'running', tone: 'neutral' },
  MET: { label: 'met', tone: 'good' },
  LATE: { label: 'met late', tone: 'warn' },
  BREACHED: { label: 'breached', tone: 'bad' },
  VOID: { label: 'no longer applies', tone: 'muted' },
}

const KIND: Record<Deadline['kind'], string> = {
  REFUND: 'Make the customer whole',
  ACKNOWLEDGE: 'Acknowledge the dispute',
  RESOLUTION: 'Resolve the dispute',
}

/** A coloured word for where a clock stands; the same vocabulary on the list and the detail page. */
export const DeadlineBadge = ({ status }: { status: Deadline['status'] }) => {
  const s = STATUS[status]
  return <Badge tone={s.tone}>{s.label}</Badge>
}

/** Days left, or days over, as a person would say it; whole days because the deadline is an end of day. */
export const remaining = (dueAt: string, now = Date.now()): string => {
  const diff = new Date(dueAt).getTime() - now
  if (diff >= 0) {
    const days = Math.ceil(diff / 86_400_000)
    return days > 1 ? `${days} days left` : 'due today'
  }
  const over = Math.max(1, Math.ceil(-diff / 86_400_000))
  return `${over} day${over === 1 ? '' : 's'} over`
}

/** The regime's clocks for one dispute, with the legal basis under each so an analyst never has to look it up. */
export const Deadlines = ({ deadlines }: { deadlines: Deadline[] }) => {
  if (deadlines.length === 0)
    return <p className="text-sm text-neutral-600">This dispute predates the clocks.</p>
  return (
    <ul className="divide-y divide-neutral-200 text-sm" aria-label="Regulatory clocks">
      {deadlines.map((d) => {
        const open = d.status === 'RUNNING' || d.status === 'BREACHED'
        return (
          <li key={`${d.kind}-${d.cycle}`} className="flex flex-wrap items-baseline gap-x-4 gap-y-1 py-2">
            <span className="w-56 font-medium">
              {KIND[d.kind]}
              {d.cycle > 0 && <span className="ml-1 text-neutral-500">(appeal {d.cycle})</span>}
            </span>
            <DeadlineBadge status={d.status} />
            <span className="text-neutral-600">
              due {d.dueAt.slice(0, 10)}
              {open && <> ({remaining(d.dueAt)})</>}
              {d.metAt && <> , met {d.metAt.slice(0, 10)}</>}
            </span>
            <span className="basis-full text-xs text-neutral-500">{d.basis}</span>
          </li>
        )
      })}
    </ul>
  )
}
