import { Link } from '@tanstack/react-router'

import type { Dispute } from '#/api/views'
import { Badge } from '#/components/ui/badge'

type Notice = Dispute['notices'][number]

export const NOTICE_KIND_LABEL: Record<Notice['kind'], string> = {
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

export const Notices = ({
  notices,
  tenant,
  disputeId,
}: {
  notices: Notice[]
  tenant: string
  disputeId: string
}) => {
  if (notices.length === 0) return <p className="text-sm text-muted-foreground">Nothing has been sent yet.</p>
  return (
    <table className="w-full text-left text-sm" aria-label="Communications">
      <thead className="text-muted-foreground">
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
        {notices.map((notice) => (
          <tr key={notice.id} className="border-t border-border">
            <td className="py-1 pr-4">{NOTICE_KIND_LABEL[notice.kind]}</td>
            <td className="py-1 pr-4 font-mono text-xs">{notice.channel}</td>
            <td className="py-1 pr-4 text-muted-foreground">{notice.recipient}</td>
            <td className="py-1 pr-4">
              <NoticeStatusBadge notice={notice} />
            </td>
            <td className="py-1">
              <Link
                to="/$tenant/disputes/$disputeId/notices/$noticeId"
                params={{ tenant, disputeId, noticeId: String(notice.id) }}
                className="underline"
              >
                {notice.channel === 'LETTER' ? 'Open letter' : 'Read'}
              </Link>
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

export const NoticeStatusBadge = ({ notice }: { notice: Notice }) => {
  if (notice.sentAt)
    return (
      <Badge tone="good">
        {notice.channel === 'LETTER'
          ? 'ready to print'
          : `sent ${notice.sentAt.slice(0, 16).replace('T', ' ')}`}
      </Badge>
    )
  if (notice.error)
    return (
      <Badge tone="warn" title={notice.error}>
        retrying
      </Badge>
    )
  return <Badge tone="neutral">queued</Badge>
}
