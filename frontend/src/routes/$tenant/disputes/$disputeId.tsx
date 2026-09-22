import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import { classify } from '#/api/failure'
import type { Dispute } from '#/api/views'
import { Deadlines } from '#/components/disputes/deadlines'
import { EventLog } from '#/components/disputes/event-log'
import { Ledger } from '#/components/disputes/ledger'
import { Notices } from '#/components/disputes/notices'
import { QuestionnairePanel } from '#/components/disputes/questionnaire'
import { RiskPanel } from '#/components/disputes/risk'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner, FieldError } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'
import { cn } from '#/lib/cn'
import { formatMoney } from '#/lib/money'
import { disputeQuery } from '#/queries'
import { fieldsOf, useServerMutation } from '#/queries/mutation'
import { applyEvent } from '#/server/functions/mutations'

type Event = Dispute['allowedEvents'][number]

const DisputePage = () => {
  const { tenant, disputeId } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: outcome } = useSuspenseQuery(disputeQuery(disputeId))
  const queryClient = useQueryClient()
  // Facts an action may carry: the customer's share of a refund, why a HIGH-risk credit goes out, how an advance clears.
  const [liability, setLiability] = useState('')
  const [settlement, setSettlement] = useState('')
  const [riskOverride, setRiskOverride] = useState('')
  const [busyEvent, setBusyEvent] = useState<Event | null>(null)

  // The idempotency key makes the call safe to repeat, so a short outage or budget hit is retried once for the person.
  const apply = useServerMutation(
    (vars: { event: Event; payload: Record<string, unknown> }) =>
      applyEvent({
        data: { disputeId, body: { event: vars.event, actor: 'analyst', payload: vars.payload } },
      }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: () => {
        setLiability('')
        setSettlement('')
        setRiskOverride('')
      },
    },
  )
  const fields = fieldsOf(apply.failure)

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <AppShell title="Dispute">
        {f && (
          <FailureBanner
            failure={f}
            onRetry={() => void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })}
          />
        )}
      </AppShell>
    )
  }
  const d = outcome.value
  const canRefund = d.allowedEvents.includes('ISSUE_REFUND')
  const canSettle = d.allowedEvents.includes('CLOSE') && Number(d.balances.suspense) > 0

  const run = (event: Event, extra: Record<string, unknown> = {}) => {
    const payload: Record<string, unknown> = { ...extra }
    if (event === 'ISSUE_REFUND' && liability.trim()) payload.liability = liability.trim()
    if (event === 'ISSUE_REFUND' && riskOverride.trim()) payload.riskOverride = riskOverride.trim()
    if (event === 'CLOSE' && settlement) payload.settlement = settlement
    setBusyEvent(event)
    apply.mutate(
      { event, payload },
      {
        onSettled: (_data, error) => {
          setBusyEvent(null)
          // A conflict means the state moved under us; show the truth alongside the message.
          if (error && apply.failure?.kind === 'conflict')
            void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })
        },
      },
    )
  }
  const fieldFailure = Boolean(
    fields.liability ||
    fields.riskOverride ||
    fields.settlement ||
    Object.keys(fields).some((k) => k.startsWith('answers.')),
  )

  return (
    <AppShell title={`Dispute ${d.id.slice(0, 8)}`}>
      <dl className="mb-6 grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
        <Field label="State" value={d.state} mono />
        <Field label="Regime" value={d.regime} mono />
        <Field label="Reason" value={d.reason} mono />
        <Field label="Amount" value={formatMoney(d.disputedAmount, d.currency)} />
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
            busy={apply.isPending}
            onReceive={(answers) => run('RECEIVE_QUESTIONNAIRE', { answers })}
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
                <Button
                  key={ev}
                  variant="action"
                  size="sm"
                  disabled={apply.isPending}
                  onClick={() => run(ev)}
                >
                  {busyEvent === ev && apply.isPending ? '...' : ev}
                </Button>
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
                  className={cn(inputVariants({ invalid: Boolean(fields.liability), mono: true }), 'w-40')}
                  {...invalidProps('liability', fields.liability)}
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
                  className={cn(inputVariants({ invalid: Boolean(fields.riskOverride) }), 'max-w-xl')}
                  {...invalidProps('riskOverride', fields.riskOverride)}
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
                  className={cn(inputVariants({ invalid: Boolean(fields.settlement) }), 'w-auto')}
                  {...invalidProps('settlement', fields.settlement)}
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
        {apply.failure && !fieldFailure && (
          <div className="mt-4">
            <FailureBanner failure={apply.failure} onRetry={apply.clearFailure} />
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Ledger</h2>
        <Ledger ledger={d.ledger} balances={d.balances} currency={d.currency} />
      </section>

      <section className="mb-8">
        <div className="mb-2 flex items-baseline justify-between">
          <h2 className="text-lg font-medium">Communications</h2>
          <Link
            to="/$tenant/disputes/$disputeId/communications"
            params={{ tenant, disputeId: d.id }}
            className="text-sm underline"
          >
            Open the communications panel
          </Link>
        </div>
        <Notices notices={d.notices} tenant={tenant} disputeId={d.id} />
      </section>

      <section>
        <h2 className="mb-2 text-lg font-medium">Event log</h2>
        <EventLog events={d.events} />
      </section>
    </AppShell>
  )
}

const Field = ({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) => (
  <div>
    <dt className="text-neutral-500">{label}</dt>
    <dd className={cn(mono && 'font-mono')}>{value}</dd>
  </div>
)

// First paint comes from the server with the session's token; actions go straight from the browser to the API.
export const Route = createFileRoute('/$tenant/disputes/$disputeId')({
  loader: ({ params, context }) => context.queryClient.ensureQueryData(disputeQuery(params.disputeId)),
  component: DisputePage,
})
