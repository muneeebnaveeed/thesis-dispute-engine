import { createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import { call } from '#/api/call'
import { classify, type Failure } from '#/api/failure'
import { AppShell } from '#/components/app-shell'
import { Deadlines } from '#/components/deadlines'
import { EventLog } from '#/components/event-log'
import { FailureBanner, FieldError } from '#/components/failure-banner'
import { Ledger } from '#/components/ledger'
import { Notices } from '#/components/notices'
import { QuestionnairePanel } from '#/components/questionnaire'
import { RiskPanel } from '#/components/risk'
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
  // Facts an action may carry: the customer's share of a refund, and how an outstanding advance clears on close.
  const [liability, setLiability] = useState('')
  const [settlement, setSettlement] = useState('')
  const [riskOverride, setRiskOverride] = useState('')

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
  const canRefund = d.allowedEvents.includes('ISSUE_REFUND')
  const canSettle = d.allowedEvents.includes('CLOSE') && Number(d.balances.suspense) > 0
  // Every fact an action carries lives under payload; the inputs are named without that prefix.
  const fields: Record<string, string> = Object.fromEntries(
    Object.entries(failure?.kind === 'validation' ? failure.fields : {}).map(([k, v]) => [
      k.replace(/^payload\./, ''),
      v,
    ]),
  )

  // The idempotency key makes the call safe to repeat, so a short outage or budget hit is retried once for the person.
  async function apply(event: Dispute['allowedEvents'][number], extra: Record<string, unknown> = {}) {
    setBusy(event)
    setFailure(null)
    const key = crypto.randomUUID()
    const payload: Record<string, unknown> = { ...extra }
    if (event === 'ISSUE_REFUND' && liability.trim()) payload.liability = liability.trim()
    if (event === 'ISSUE_REFUND' && riskOverride.trim()) payload.riskOverride = riskOverride.trim()
    if (event === 'CLOSE' && settlement) payload.settlement = settlement
    const res = await call(
      () =>
        api.POST('/disputes/{disputeId}/events', {
          params: { path: { disputeId: d.id } },
          body: { event, actor: 'analyst', payload },
          headers: { 'Idempotency-Key': key },
        }),
      { idempotent: true },
    )
    setBusy(null)
    if (res.failure) {
      setFailure(res.failure)
      // A conflict means the state moved under us; show the truth alongside the message.
      if (res.failure.kind === 'conflict') await router.invalidate()
    } else {
      setLiability('')
      setSettlement('')
      setRiskOverride('')
      await router.invalidate()
    }
  }

  return (
    <AppShell title={`Dispute ${d.id.slice(0, 8)}`}>
      <dl className="mb-6 grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
        <Field label="State" value={d.state} mono />
        <Field label="Regime" value={d.regime} mono />
        <Field label="Reason" value={d.reason} mono />
        <Field label="Amount" value={`${d.disputedAmount} ${d.currency}`} />
        <Field label="Appeals used" value={String(d.appeals)} />
      </dl>

      {d.risk && (
        <section className="mb-8">
          <h2 className="mb-2 text-lg font-medium">Fraud risk</h2>
          <RiskPanel risk={d.risk} />
        </section>
      )}

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Regulatory clocks</h2>
        <Deadlines deadlines={d.deadlines} />
      </section>

      {d.questionnaire && (
        <section className="mb-8">
          <h2 className="mb-2 text-lg font-medium">Questionnaire</h2>
          <QuestionnairePanel
            questionnaire={d.questionnaire}
            canReceive={d.allowedEvents.includes('RECEIVE_QUESTIONNAIRE')}
            fields={fields}
            busy={busy !== null}
            onReceive={(answers) => void apply('RECEIVE_QUESTIONNAIRE', { answers })}
          />
        </section>
      )}

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Actions</h2>
        {d.allowedEvents.length === 0 ? (
          <p className="text-sm text-neutral-600">This dispute is closed; nothing more can happen to it.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {d.allowedEvents
              .filter((ev) => ev !== 'RECEIVE_QUESTIONNAIRE')
              .map((ev) => (
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
        {(canRefund || canSettle) && (
          <div className="mt-3 flex flex-wrap gap-6 text-sm">
            {canRefund && (
              <label className="block">
                Customer liability ({d.currency}, optional)
                <input
                  value={liability}
                  onChange={(e) => setLiability(e.target.value)}
                  inputMode="decimal"
                  placeholder="0.00"
                  aria-invalid={fields.liability ? true : undefined}
                  aria-describedby={fields.liability ? 'liability-error' : undefined}
                  className={`mt-1 block w-40 rounded-md border px-2 py-1 font-mono ${fields.liability ? 'border-red-400' : 'border-neutral-300'}`}
                />
                <FieldError id="liability-error" message={fields.liability} />
              </label>
            )}
            {canRefund && d.risk?.tier === 'HIGH' && (
              <label className="block basis-full">
                Justification for crediting despite the HIGH risk score
                <textarea
                  value={riskOverride}
                  onChange={(e) => setRiskOverride(e.target.value)}
                  rows={2}
                  aria-invalid={fields.riskOverride ? true : undefined}
                  aria-describedby={fields.riskOverride ? 'riskOverride-error' : undefined}
                  className={`mt-1 block w-full max-w-xl rounded-md border px-2 py-1 ${fields.riskOverride ? 'border-red-400' : 'border-neutral-300'}`}
                />
                <FieldError id="riskOverride-error" message={fields.riskOverride} />
              </label>
            )}
            {canSettle && (
              <label className="block">
                Outstanding advance on close
                <select
                  value={settlement}
                  onChange={(e) => setSettlement(e.target.value)}
                  aria-invalid={fields.settlement ? true : undefined}
                  className="mt-1 block rounded-md border border-neutral-300 px-2 py-1"
                >
                  <option value="">regime default</option>
                  <option value="RECOVERED">recovered</option>
                  <option value="WRITTEN_OFF">written off</option>
                </select>
                <FieldError id="settlement-error" message={fields.settlement} />
              </label>
            )}
          </div>
        )}
        {failure &&
          !(
            fields.liability ||
            fields.riskOverride ||
            fields.settlement ||
            Object.keys(fields).some((k) => k.startsWith('answers.'))
          ) && (
            <div className="mt-4">
              <FailureBanner failure={failure} onRetry={() => setFailure(null)} />
            </div>
          )}
      </section>

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Ledger</h2>
        <Ledger ledger={d.ledger} balances={d.balances} currency={d.currency} />
      </section>

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Communications</h2>
        <Notices notices={d.notices} tenant={tenant} disputeId={d.id} />
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
