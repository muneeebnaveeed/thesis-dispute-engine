import { createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import { call } from '#/api/call'
import { classify, type Failure } from '#/api/failure'
import { AppShell } from '#/components/app-shell'
import { EventLog } from '#/components/event-log'
import { FailureBanner } from '#/components/failure-banner'
import { TenantMismatch } from '#/components/tenant-mismatch'
import { getDispute, type Dispute } from '#/server/disputes'

// First paint comes from the server with the session's token; actions go straight from the browser to the API.
export const Route = createFileRoute('/$tenant/disputes/$disputeId')({
  loader: ({ params }) => getDispute({ data: params.disputeId }),
  component: DisputePage,
})

function DisputePage() {
  const outcome = Route.useLoaderData()
  const { tenant } = Route.useParams()
  const { config, viewer } = useRouteContext({ from: '__root__' })
  const api = useMemo(
    () =>
      createBrowserApi(config.apiUrl, () =>
        window.location.assign(`/${tenant}?next=${encodeURIComponent(window.location.pathname)}`),
      ),
    [config.apiUrl, tenant],
  )
  const router = useRouter()
  const [failure, setFailure] = useState<Failure | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <AppShell title="Dispute">
        {f && <FailureBanner failure={f} onRetry={() => void router.invalidate()} />}
      </AppShell>
    )
  }
  const d = outcome.value

  // The idempotency key makes the call safe to repeat, so a short outage or budget hit is retried once for the person.
  async function apply(event: Dispute['allowedEvents'][number]) {
    setBusy(event)
    setFailure(null)
    const key = crypto.randomUUID()
    const res = await call(
      () =>
        api.POST('/disputes/{disputeId}/events', {
          params: { path: { disputeId: d.id } },
          body: { event, actor: 'analyst' },
          headers: { 'Idempotency-Key': key },
        }),
      { idempotent: true },
    )
    setBusy(null)
    if (res.failure) {
      setFailure(res.failure)
      // A conflict means the state moved under us; show the truth alongside the message.
      if (res.failure.kind === 'conflict') await router.invalidate()
    } else await router.invalidate()
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
        {failure && (
          <div className="mt-4">
            <FailureBanner failure={failure} onRetry={() => setFailure(null)} />
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
