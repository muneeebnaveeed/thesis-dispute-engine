import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'

import { classify } from '#/api/failure'
import { FailureBanner } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { noticeQuery } from '#/queries'

// A letter on its own page, laid out to print: no shell, every value rendered as text.
const NoticePage = () => {
  const { tenant, disputeId, noticeId } = Route.useParams()
  const id = Number(noticeId)
  const { data: outcome } = useSuspenseQuery(noticeQuery(disputeId, id))
  const queryClient = useQueryClient()
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <main className="mx-auto max-w-2xl p-8">
        {f && (
          <FailureBanner
            failure={f}
            onRetry={() =>
              void queryClient.invalidateQueries({ queryKey: noticeQuery(disputeId, id).queryKey })
            }
          />
        )}
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
        <Button variant="link" size="bare" onClick={() => window.print()}>
          Print
        </Button>
      </nav>
      <header className="mb-8 text-neutral-600">
        <p>{n.bank}</p>
        <p>{n.date.slice(0, 10)}</p>
        <p className="mt-4 whitespace-pre-line">{n.recipient}</p>
      </header>
      <h1 className="mb-6 text-[14pt] font-semibold">{n.subject}</h1>
      <p className="mb-4">{n.greeting}</p>
      {n.paragraphs.map((p, i) => (
        <p
          // Paragraphs are prose from the API; position is the only identity they have.
          // eslint-disable-next-line react/no-array-index-key
          key={i}
          className="mb-4 whitespace-pre-line"
        >
          {p}
        </p>
      ))}
      <p className="mb-4 whitespace-pre-line">{n.closing}</p>
      {n.basis && <p className="mt-12 text-[10pt] text-neutral-600">{n.basis}</p>}
    </main>
  )
}

export const Route = createFileRoute('/$tenant/disputes/$disputeId_/notices/$noticeId')({
  loader: ({ params, context }) =>
    context.queryClient.query({
      ...noticeQuery(params.disputeId, Number(params.noticeId)),
      staleTime: 'static',
    }),
  component: NoticePage,
})
