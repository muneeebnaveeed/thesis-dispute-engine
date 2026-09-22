import { Link } from '@tanstack/react-router'

import type { Dispute } from '#/api/views'

type Notice = Dispute['notices'][number]

const KIND: Record<Notice['kind'], string> = {
  ACKNOWLEDGEMENT: 'Acknowledgement',
  QUESTIONNAIRE: 'Questionnaire',
  PROVISIONAL_CREDIT: 'Provisional credit notice',
  REFUND: 'Refund notice',
  REVERSAL: 'Reversal notice',
  RESOLUTION: 'Resolution',
  REQUEST_FOR_INFORMATION: 'Request for information',
  STATUS_UPDATE: 'Status update',
  DOCUMENTS_RECEIVED: 'Documents received',
  CUSTOM: 'Custom message',
}

/** Every communication owed to the customer: what, how, to whom, and whether it has gone. */
export const Notices = ({
  notices,
  tenant,
  disputeId,
}: {
  notices: Notice[]
  tenant: string
  disputeId: string
}) => {
  if (notices.length === 0) return <p className="text-sm text-neutral-600">Nothing has been sent yet.</p>
  return (
    <table className="w-full text-left text-sm" aria-label="Communications">
      <thead className="text-neutral-500">
        <tr>
          <th className="py-1 pr-4 font-normal">Notice</th>
          <th className="py-1 pr-4 font-normal">Channel</th>
          <th className="py-1 pr-4 font-normal">To</th>
          <th className="py-1 pr-4 font-normal">Status</th>
          <th className="py-1 font-normal">
            <span className="sr-only">Open</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {notices.map((n) => (
          <tr key={n.id} className="border-t border-neutral-200">
            <td className="py-1 pr-4">{KIND[n.kind]}</td>
            <td className="py-1 pr-4 font-mono text-xs">{n.channel}</td>
            <td className="py-1 pr-4 text-neutral-600">{n.recipient}</td>
            <td className="py-1 pr-4">
              <Status notice={n} />
            </td>
            <td className="py-1">
              <Link
                to="/$tenant/disputes/$disputeId/notices/$noticeId"
                params={{ tenant, disputeId, noticeId: String(n.id) }}
                className="underline"
              >
                {n.channel === 'LETTER' ? 'Open letter' : 'Read'}
              </Link>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

const Status = ({ notice: n }: { notice: Notice }) => {
  if (n.sentAt)
    return (
      <span className="rounded bg-emerald-100 px-1.5 py-0.5 text-xs font-medium text-emerald-900">
        {n.channel === 'LETTER' ? 'ready to print' : `sent ${n.sentAt.slice(0, 16).replace('T', ' ')}`}
      </span>
    )
  if (n.error)
    return (
      <span className="rounded bg-amber-100 px-1.5 py-0.5 text-xs font-medium text-amber-900" title={n.error}>
        retrying
      </span>
    )
  return (
    <span className="rounded bg-neutral-100 px-1.5 py-0.5 text-xs font-medium text-neutral-800">queued</span>
  )
}
