import { useState } from 'react'

import { useQuery } from '@tanstack/react-query'

import type { Dispute } from '#/api/views'
import { noticeQuery } from '#/queries'
import { getAttachment } from '#/server/functions/reads'
import { cn } from '#/lib/cn'

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

/** Sent Emails: every communication on the dispute, newest first, with the selected one shown as the customer read it. */
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
  const rows = notices.toSorted((a, b) => b.id - a.id)
  // The newest notice is shown until the analyst picks another, so a just-sent email opens selected.
  const [picked, setSelected] = useState<number | null>(null)
  const selected = picked ?? rows[0]?.id ?? null
  // The document of the selected notice; the query keeps earlier ones cached, so switching back is instant.
  const { data: docOutcome } = useQuery({
    ...noticeQuery(disputeId, selected ?? 0),
    enabled: selected !== null,
  })
  const doc = docOutcome?.value ?? null
  const selectedNotice = rows.find((n) => n.id === selected)

  // The file comes back through the same bearer as everything else, then opens in a new tab as a blob.
  // The file comes through the server function as base64 and is saved from a blob; nothing calls the API directly.
  const openAttachment = async (id: string, filename: string) => {
    const res = await getAttachment({ data: { disputeId, attachmentId: id } })
    if (!res.value) return
    const bytes = Uint8Array.from(atob(res.value.base64), (c) => c.charCodeAt(0))
    const url = URL.createObjectURL(new Blob([bytes], { type: res.value.contentType }))
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    setTimeout(() => URL.revokeObjectURL(url), 10_000)
  }

  if (rows.length === 0) return <p className="text-sm text-neutral-600">Nothing has been sent yet.</p>
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
          {rows.map((n) => (
            <tr
              key={n.id}
              aria-selected={n.id === selected}
              onClick={() => setSelected(n.id)}
              className={cn(
                'cursor-pointer border-t border-neutral-200',
                n.id === selected ? 'bg-neutral-100' : 'hover:bg-neutral-50',
              )}
            >
              <td className="py-1 pr-4">
                <button type="button" className="text-left" onClick={() => setSelected(n.id)}>
                  {n.resendOf !== undefined && <span className="text-neutral-500">Resent: </span>}
                  {KIND[n.kind]}
                </button>
              </td>
              <td className="py-1 pr-4 font-mono text-xs">{n.channel}</td>
              <td className="py-1 pr-4 text-neutral-600">{n.actor ?? 'engine'}</td>
              <td className="py-1 pr-4">
                <Status notice={n} />
              </td>
              <td className="py-1">
                {onResend && n.channel === 'EMAIL' && n.sentAt && (
                  <button
                    type="button"
                    disabled={busy}
                    className="text-xs underline disabled:opacity-50"
                    onClick={(e) => {
                      e.stopPropagation()
                      onResend(n.id)
                    }}
                  >
                    Resend
                  </button>
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
        {doc ? (
          <>
            <p className="mb-6 text-sm text-neutral-500">
              {doc.bank}
              <br />
              {doc.date.slice(0, 10)}
              <br />
              To: {doc.recipient}
            </p>
            <h3 className="mb-4 text-[14pt] font-semibold">{doc.subject}</h3>
            <p className="mb-4">{doc.greeting}</p>
            {doc.paragraphs.map((p, i) => (
              // eslint-disable-next-line react/no-array-index-key
              <p key={i} className="mb-4 whitespace-pre-line">
                {p}
              </p>
            ))}
            <p className="whitespace-pre-line">{doc.closing}</p>
            {doc.basis && <p className="mt-8 text-[10pt] text-neutral-600">{doc.basis}</p>}
            {selectedNotice && selectedNotice.attachments.length > 0 && (
              <ul className="mt-6 space-y-1 font-sans text-sm" aria-label="Attachments">
                {selectedNotice.attachments.map((a) => (
                  <li key={a.id}>
                    <button
                      type="button"
                      className="underline"
                      onClick={() => void openAttachment(a.id, a.filename)}
                    >
                      {a.filename}
                    </button>{' '}
                    <span className="text-neutral-500">({Math.ceil(a.size / 1024)} KB)</span>
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
