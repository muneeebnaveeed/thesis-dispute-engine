import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute } from '@tanstack/react-router'

import { FailureBanner } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { noticeQuery } from '#/queries/notices'

const NoticePage = () => {
  const { tenant, disputeId, noticeId } = Route.useParams()
  const noticeNumber = Number(noticeId)
  const { data: loadedNotice } = useSuspenseQuery(noticeQuery(disputeId, noticeNumber))
  const queryClient = useQueryClient()
  if (loadedNotice.failure) {
    return (
      <main className="mx-auto max-w-2xl p-8">
        <FailureBanner
          failure={loadedNotice.failure}
          onRetry={() =>
            void queryClient.invalidateQueries({ queryKey: noticeQuery(disputeId, noticeNumber).queryKey })
          }
        />
      </main>
    )
  }
  const letter = loadedNotice.value
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
        <p>{letter.bank}</p>
        <p>{letter.date.slice(0, 10)}</p>
        <p className="mt-4 whitespace-pre-line">{letter.recipient}</p>
      </header>
      <h1 className="mb-6 text-[14pt] font-semibold">{letter.subject}</h1>
      <p className="mb-4">{letter.greeting}</p>
      {letter.paragraphs.map((paragraph, paragraphIndex) => (
        // eslint-disable-next-line react/no-array-index-key -- paragraphs are positional prose
        <p key={paragraphIndex} className="mb-4 whitespace-pre-line">
          {paragraph}
        </p>
      ))}
      <p className="mb-4 whitespace-pre-line">{letter.closing}</p>
      {letter.basis && <p className="mt-12 text-[10pt] text-neutral-600">{letter.basis}</p>}
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
