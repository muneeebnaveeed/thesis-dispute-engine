import { Link } from '@tanstack/react-router'

import type { Dispute } from '#/api/views'
import { Badge } from '#/components/ui/badge'
import { gridBody, gridHead, gridTable } from '#/components/ui/grid'

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
  if (notices.length === 0)
    return <p className="text-[11px] text-muted-foreground">Nothing has been sent yet.</p>
  return (
    <table className={gridTable} aria-label="Communications">
      <thead className={gridHead}>
        <tr>
          <th>Notice</th>
          <th>Channel</th>
          <th>To</th>
          <th>Status</th>
          <th>
            <span className="sr-only">Open</span>
          </th>
        </tr>
      </thead>
      <tbody className={gridBody}>
        {notices.map((notice) => (
          <tr key={notice.id} className="hover:bg-accent">
            <td>{NOTICE_KIND_LABEL[notice.kind]}</td>
            <td className="font-mono">{notice.channel}</td>
            <td className="text-muted-foreground">{notice.recipient}</td>
            <td>
              <NoticeStatusBadge notice={notice} />
            </td>
            <td>
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
