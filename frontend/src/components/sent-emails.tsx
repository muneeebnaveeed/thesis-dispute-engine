import { useEffect, useState } from 'react'

import type { Api } from '#/api/client'
import { call } from '#/api/call'
import type { components } from '#/api/schema.gen'
import type { Dispute } from '#/server/disputes'

type Notice = Dispute['notices'][number]
type NoticeDocument = components['schemas']['NoticeDocument']

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
export function SentEmails({
  notices,
  api,
  disputeId,
  onResend,
  busy = false,
}: {
  notices: Notice[]
  api: Api
  disputeId: string
  onResend?: (noticeId: number) => void
  busy?: boolean
}) {
  const rows = notices.toSorted((a, b) => b.id - a.id)
  // The newest notice is shown until the analyst picks another, so a just-sent email opens selected.
  const [picked, setSelected] = useState<number | null>(null)
  const selected = picked ?? rows[0]?.id ?? null
  const doc = useNotice(api, disputeId, selected)
  const selectedNotice = rows.find((n) => n.id === selected)

  // The file comes back through the same bearer as everything else, then opens in a new tab as a blob.
  async function openAttachment(id: string, filename: string) {
    const res = await api.GET('/disputes/{disputeId}/attachments/{attachmentId}', {
      params: { path: { disputeId, attachmentId: id } },
      parseAs: 'blob',
    })
    if (!res.data) return
    const url = URL.createObjectURL(res.data)
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
              className={`cursor-pointer border-t border-neutral-200 ${n.id === selected ? 'bg-neutral-100' : 'hover:bg-neutral-50'}`}
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

/** The selected notice's document; the previous one stays visible until the next arrives. */
function useNotice(api: Api, disputeId: string, id: number | null): NoticeDocument | null {
  const [docs, setDocs] = useState<Record<number, NoticeDocument>>({})
  useEffect(() => {
    if (id === null || docs[id]) return undefined
    let live = true
    call(() =>
      api.GET('/disputes/{disputeId}/notices/{noticeId}', { params: { path: { disputeId, noticeId: id } } }),
    )
      .then((res) => {
        if (live && res.data) setDocs((cur) => ({ ...cur, [id]: res.data }))
        return undefined
      })
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [api, disputeId, id, docs])
  return id === null ? null : (docs[id] ?? null)
}

function Status({ notice: n }: { notice: Notice }) {
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
