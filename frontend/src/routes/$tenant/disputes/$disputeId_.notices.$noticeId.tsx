import { Link, createFileRoute, useRouter } from '@tanstack/react-router'

import { classify } from '#/api/failure'
import { FailureBanner } from '#/components/failure-banner'
import { getNotice } from '#/server/notices'

// A letter on its own page, laid out to print: no shell, no scripts needed, every value rendered as text.
export const Route = createFileRoute('/$tenant/disputes/$disputeId_/notices/$noticeId')({
  loader: ({ params }) =>
    getNotice({ data: { disputeId: params.disputeId, noticeId: Number(params.noticeId) } }),
  component: NoticePage,
})

function NoticePage() {
  const outcome = Route.useLoaderData()
  const { tenant, disputeId } = Route.useParams()
  const router = useRouter()
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <main className="mx-auto max-w-2xl p-8">
        {f && <FailureBanner failure={f} onRetry={() => void router.invalidate()} />}
      </main>
    )
  }
  const n = outcome.value
  return (
    <main className="mx-auto max-w-2xl p-8 font-serif text-[12pt] leading-relaxed text-neutral-900 print:p-0">
      <nav className="mb-8 flex gap-4 font-sans text-sm print:hidden">
        <Link to="/$tenant/disputes/$disputeId" params={{ tenant, disputeId }} className="underline">
          Back to the dispute
        </Link>
        <button type="button" className="underline" onClick={() => window.print()}>
          Print
        </button>
      </nav>
      <header className="mb-8 text-neutral-600">
        <p>{n.bank}</p>
        <p>{n.date.slice(0, 10)}</p>
        <p className="mt-4 whitespace-pre-line">{n.recipient}</p>
      </header>
      <h1 className="mb-6 text-[14pt] font-semibold">{n.subject}</h1>
      <p className="mb-4">{n.greeting}</p>
      {n.paragraphs.map((p, i) => (
        // Paragraphs are prose from the API; position is the only identity they have.
        // eslint-disable-next-line react/no-array-index-key
        <p key={i} className="mb-4">
          {p}
        </p>
      ))}
      <p className="mb-4 whitespace-pre-line">{n.closing}</p>
      {n.basis && <p className="mt-12 text-[10pt] text-neutral-600">{n.basis}</p>}
    </main>
  )
}
