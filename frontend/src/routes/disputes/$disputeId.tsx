import { createFileRoute, redirect, useRouteContext, useRouter } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import type { Problem } from '#/api/problem'
import { AppShell } from '#/components/app-shell'
import { EventLog } from '#/components/event-log'
import { ProblemBanner } from '#/components/problem-banner'
import { getDispute, type Dispute } from '#/server/disputes'

// First paint comes from the server with the session's token; actions go straight from the browser to the API.
export const Route = createFileRoute('/disputes/$disputeId')({
  // Anonymous visitors go to sign-in and come back here afterwards (the destination rides along as ?next).
  beforeLoad: ({ context, location }) => {
    if (!context.viewer) throw redirect({ to: '/', search: { next: location.href } })
  },
  loader: ({ params }) => getDispute({ data: params.disputeId }),
  component: DisputePage,
})

function DisputePage() {
  const outcome = Route.useLoaderData()
  const { config } = useRouteContext({ from: '__root__' })
  const api = useMemo(() => createBrowserApi(config.apiUrl), [config.apiUrl])
  const router = useRouter()
  const [problem, setProblem] = useState<Problem | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  if (outcome.problem || !outcome.value) {
    return (
      <AppShell title="Dispute">{outcome.problem && <ProblemBanner problem={outcome.problem} />}</AppShell>
    )
  }
  const d = outcome.value

  async function apply(event: Dispute['allowedEvents'][number]) {
    setBusy(event)
    setProblem(null)
    const { error } = await api.POST('/disputes/{disputeId}/events', {
      params: { path: { disputeId: d.id } },
      body: { event, actor: 'analyst' },
      headers: { 'Idempotency-Key': crypto.randomUUID() },
    })
    setBusy(null)
    if (error) setProblem(error)
    else await router.invalidate()
  }

  return (
    <AppShell title={`Dispute ${d.id.slice(0, 8)}`}>
      <dl className="mb-6 grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
        <Field label="State" value={d.state} mono />
        <Field label="Regime" value={d.regime} mono />
        <Field label="Amount" value={`${d.disputedAmount} ${d.currency}`} />
        <Field label="Appeals used" value={String(d.appeals)} />
      </dl>

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Actions</h2>
        {d.allowedEvents.length === 0 ? (
          <p className="text-sm text-neutral-600">This dispute is closed; nothing more can happen to it.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {d.allowedEvents.map((ev) => (
              <button
                key={ev}
                type="button"
                disabled={busy !== null}
                onClick={() => void apply(ev)}
                className="rounded-md border border-neutral-300 bg-white px-3 py-1.5 font-mono text-sm hover:bg-neutral-100 disabled:opacity-50"
              >
                {busy === ev ? '...' : ev}
              </button>
            ))}
          </div>
        )}
        {problem && (
          <div className="mt-4">
            <ProblemBanner problem={problem} />
          </div>
        )}
      </section>

      <section>
        <h2 className="mb-2 text-lg font-medium">Event log</h2>
        <EventLog events={d.events} />
      </section>
    </AppShell>
  )
}

function Field({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) {
  return (
    <div>
      <dt className="text-neutral-500">{label}</dt>
      <dd className={mono ? 'font-mono' : ''}>{value}</dd>
    </div>
  )
}
