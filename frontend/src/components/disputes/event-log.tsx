import type { Dispute } from '#/api/views'
import { gridBody, gridHead, gridTable } from '#/components/ui/grid'

export const EventLog = ({ events }: { events: Dispute['events'] }) => {
  const newestFirst = events.toSorted((earlier, later) => later.seq - earlier.seq)
  return (
    <table className={gridTable} aria-label="Event log">
      <thead className={gridHead}>
        <tr>
          <th>#</th>
          <th>Event</th>
          <th>From</th>
          <th>To</th>
          <th>Actor</th>
          <th>When</th>
        </tr>
      </thead>
      <tbody className={gridBody}>
        {newestFirst.map((transition) => (
          <tr key={transition.seq} className="font-mono">
            <td className="text-muted-foreground">{transition.seq}</td>
            <td>{transition.event}</td>
            <td className="text-muted-foreground">{transition.fromState || '-'}</td>
            <td>{transition.toState}</td>
            <td>{transition.actor}</td>
            <td className="text-muted-foreground">
              {new Date(transition.occurredAt).toISOString().replace('T', ' ').slice(0, 19)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
