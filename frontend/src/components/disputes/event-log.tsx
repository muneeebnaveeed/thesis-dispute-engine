import type { Dispute } from '#/api/views'

export const EventLog = ({ events }: { events: Dispute['events'] }) => {
  const newestFirst = events.toSorted((earlier, later) => later.seq - earlier.seq)
  return (
    <table className="w-full text-left text-sm" aria-label="Event log">
      <thead className="text-neutral-500">
        <tr>
          <th className="py-1 pr-4 font-normal">#</th>
          <th className="py-1 pr-4 font-normal">Event</th>
          <th className="py-1 pr-4 font-normal">From</th>
          <th className="py-1 pr-4 font-normal">To</th>
          <th className="py-1 pr-4 font-normal">Actor</th>
          <th className="py-1 font-normal">When</th>
        </tr>
      </thead>
      <tbody>
        {newestFirst.map((transition) => (
          <tr key={transition.seq} className="border-t border-neutral-200 font-mono">
            <td className="py-1 pr-4 text-neutral-500">{transition.seq}</td>
            <td className="py-1 pr-4">{transition.event}</td>
            <td className="py-1 pr-4 text-neutral-500">{transition.fromState || '-'}</td>
            <td className="py-1 pr-4">{transition.toState}</td>
            <td className="py-1 pr-4">{transition.actor}</td>
            <td className="py-1 text-neutral-500">
              {new Date(transition.occurredAt).toISOString().replace('T', ' ').slice(0, 19)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
