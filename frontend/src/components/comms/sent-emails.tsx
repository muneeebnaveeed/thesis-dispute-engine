import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'

import type { Dispute } from '#/api/views'
import { NOTICE_KIND_LABEL, NoticeStatusBadge } from '#/components/disputes/notices'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/cn'
import { noticeQuery } from '#/queries/notices'
import { useServerMutation } from '#/queries/use-server-mutation'
import { getAttachment } from '#/server/functions/notices'

type Notice = Dispute['notices'][number]

type AttachmentBytes = NonNullable<Awaited<ReturnType<typeof getAttachment>>['data']>

const saveToDisk = ({ base64, contentType }: AttachmentBytes, filename: string) => {
  const bytes = Uint8Array.from(atob(base64), (char) => char.charCodeAt(0))
  const objectUrl = URL.createObjectURL(new Blob([bytes], { type: contentType }))
  const downloadLink = document.createElement('a')
  downloadLink.href = objectUrl
  downloadLink.download = filename
  downloadLink.click()
  setTimeout(() => URL.revokeObjectURL(objectUrl), 10_000)
}

export const SentEmails = ({
  notices,
  disputeId,
  onResend,
  busy = false,
}: {
  notices: Notice[]
  disputeId: string
  onResend?: (noticeId: number) => void
  busy?: boolean
}) => {
  const newestFirst = notices.toSorted((earlier, later) => later.id - earlier.id)
  // newest wins until the analyst picks, so a just-sent email opens selected
  const [pickedNoticeId, setPickedNoticeId] = useState<number | null>(null)
  const selectedNoticeId = pickedNoticeId ?? newestFirst[0]?.id ?? null
  const { data: loadedDocument } = useQuery({
    ...noticeQuery(disputeId, selectedNoticeId ?? 0),
    enabled: selectedNoticeId !== null,
  })
  const noticeDocument = loadedDocument?.value ?? null
  const selectedNotice = newestFirst.find((notice) => notice.id === selectedNoticeId)
  const downloadMutation = useServerMutation(
    ({ attachmentId }: { attachmentId: string; filename: string }) =>
      getAttachment({ data: { disputeId, attachmentId } }),
    { onSuccess: (bytes, { filename }) => saveToDisk(bytes, filename) },
  )

  if (newestFirst.length === 0) return <p className="text-sm text-neutral-600">Nothing has been sent yet.</p>
  return (
    <div className="grid gap-8 lg:grid-cols-2">
      <table className="w-full self-start text-left text-sm" aria-label="Sent emails">
        <thead className="text-neutral-500">
          <tr>
            <th className="py-1 pr-4 font-normal">Notice</th>
            <th className="py-1 pr-4 font-normal">Channel</th>
            <th className="py-1 pr-4 font-normal">By</th>
            <th className="py-1 pr-4 font-normal">Status</th>
            <th className="py-1 font-normal">
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {newestFirst.map((notice) => (
            <tr
              key={notice.id}
              aria-selected={notice.id === selectedNoticeId}
              onClick={() => setPickedNoticeId(notice.id)}
              className={cn(
                'cursor-pointer border-t border-neutral-200',
                notice.id === selectedNoticeId ? 'bg-neutral-100' : 'hover:bg-neutral-50',
              )}
            >
              <td className="py-1 pr-4">
                <button type="button" className="text-left" onClick={() => setPickedNoticeId(notice.id)}>
                  {notice.resendOf !== undefined && <span className="text-neutral-500">Resent: </span>}
                  {NOTICE_KIND_LABEL[notice.kind]}
                </button>
              </td>
              <td className="py-1 pr-4 font-mono text-xs">{notice.channel}</td>
              <td className="py-1 pr-4 text-neutral-600">{notice.actor ?? 'engine'}</td>
              <td className="py-1 pr-4">
                <NoticeStatusBadge notice={notice} />
              </td>
              <td className="py-1">
                {onResend && notice.channel === 'EMAIL' && notice.sentAt && (
                  <Button
                    variant="link"
                    size="bare"
                    className="text-xs"
                    disabled={busy}
                    onClick={(event) => {
                      event.stopPropagation()
                      onResend(notice.id)
                    }}
                  >
                    Resend
                  </Button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <section
        aria-label="Sent email preview"
        className="rounded-md border border-neutral-200 bg-white p-6 font-serif text-[12pt] leading-relaxed"
      >
        {noticeDocument ? (
          <>
            <p className="mb-6 text-sm text-neutral-500">
              {noticeDocument.bank}
              <br />
              {noticeDocument.date.slice(0, 10)}
              <br />
              To: {noticeDocument.recipient}
            </p>
            <h3 className="mb-4 text-[14pt] font-semibold">{noticeDocument.subject}</h3>
            <p className="mb-4">{noticeDocument.greeting}</p>
            {noticeDocument.paragraphs.map((paragraph, paragraphIndex) => (
              // eslint-disable-next-line react/no-array-index-key -- paragraphs are positional prose
              <p key={paragraphIndex} className="mb-4 whitespace-pre-line">
                {paragraph}
              </p>
            ))}
            <p className="whitespace-pre-line">{noticeDocument.closing}</p>
            {noticeDocument.basis && (
              <p className="mt-8 text-[10pt] text-neutral-600">{noticeDocument.basis}</p>
            )}
            {selectedNotice && selectedNotice.attachments.length > 0 && (
              <ul className="mt-6 space-y-1 font-sans text-sm" aria-label="Attachments">
                {selectedNotice.attachments.map((attachment) => (
                  <li key={attachment.id}>
                    <Button
                      variant="link"
                      size="bare"
                      disabled={downloadMutation.isPending}
                      onClick={() =>
                        downloadMutation.mutate({
                          attachmentId: attachment.id,
                          filename: attachment.filename,
                        })
                      }
                    >
                      {attachment.filename}
                    </Button>{' '}
                    <span className="text-neutral-500">({Math.ceil(attachment.size / 1024)} KB)</span>
                  </li>
                ))}
              </ul>
            )}
          </>
        ) : (
          <p className="text-sm text-neutral-500">Loading...</p>
        )}
      </section>
    </div>
  )
}
