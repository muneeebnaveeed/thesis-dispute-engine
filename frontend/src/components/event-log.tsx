import type { Dispute } from '#/server/disputes'

/** The append-only log, newest first; each row is one accepted transition. */
export function EventLog({ events }: { events: Dispute['events'] }) {
  const rows = events.toSorted((a, b) => b.seq - a.seq)
  return (
    <table className="w-full text-left text-sm">
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
        {rows.map((e) => (
          <tr key={e.seq} className="border-t border-neutral-200 font-mono">
            <td className="py-1 pr-4 text-neutral-500">{e.seq}</td>
            <td className="py-1 pr-4">{e.event}</td>
            <td className="py-1 pr-4 text-neutral-500">{e.fromState || '-'}</td>
            <td className="py-1 pr-4">{e.toState}</td>
            <td className="py-1 pr-4">{e.actor}</td>
            <td className="py-1 text-neutral-500">
              {new Date(e.occurredAt).toISOString().replace('T', ' ').slice(0, 19)}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
