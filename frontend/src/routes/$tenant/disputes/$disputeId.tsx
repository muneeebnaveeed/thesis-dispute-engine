import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

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
import { disputeQuery } from '#/queries/disputes'
import { fieldErrorsOf, useServerMutation } from '#/queries/use-server-mutation'
import { applyEvent } from '#/server/functions/disputes'

type DisputeEvent = Dispute['allowedEvents'][number]

const DisputePage = () => {
  const { tenant, disputeId } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: loadedDispute } = useSuspenseQuery(disputeQuery(disputeId))
  const queryClient = useQueryClient()
  const [liability, setLiability] = useState('')
  const [settlement, setSettlement] = useState('')
  const [riskOverride, setRiskOverride] = useState('')
  const [eventInFlight, setEventInFlight] = useState<DisputeEvent | null>(null)

  const applyEventMutation = useServerMutation(
    ({ event, payload }: { event: DisputeEvent; payload: Record<string, unknown> }) =>
      applyEvent({ data: { disputeId, body: { event, actor: 'analyst', payload } } }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: () => {
        setLiability('')
        setSettlement('')
        setRiskOverride('')
      },
    },
  )
  const applyFailure = applyEventMutation.failure
  const fieldErrors = fieldErrorsOf(applyFailure)
  const refreshDispute = () =>
    void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (loadedDispute.failure) {
    return (
      <AppShell title="Dispute">
        <FailureBanner failure={loadedDispute.failure} onRetry={refreshDispute} />
      </AppShell>
    )
  }
  const dispute = loadedDispute.value
  const canRefund = dispute.allowedEvents.includes('ISSUE_REFUND')
  const canSettle = dispute.allowedEvents.includes('CLOSE') && Number(dispute.balances.suspense) > 0

  const applyWithFormFacts = (event: DisputeEvent, payloadExtras: Record<string, unknown> = {}) => {
    const payload: Record<string, unknown> = { ...payloadExtras }
    if (event === 'ISSUE_REFUND' && liability.trim()) payload.liability = liability.trim()
    if (event === 'ISSUE_REFUND' && riskOverride.trim()) payload.riskOverride = riskOverride.trim()
    if (event === 'CLOSE' && settlement) payload.settlement = settlement
    setEventInFlight(event)
    applyEventMutation.mutate(
      { event, payload },
      {
        onSettled: (_data, error) => {
          setEventInFlight(null)
          // conflict: the state moved under us, show the truth next to the message
          if (error && applyEventMutation.failure?.kind === 'conflict') refreshDispute()
        },
      },
    )
  }
  const failureShownAtField = Boolean(
    fieldErrors.liability ||
    fieldErrors.riskOverride ||
    fieldErrors.settlement ||
    Object.keys(fieldErrors).some((fieldPath) => fieldPath.startsWith('answers.')),
  )

  return (
    <AppShell title={`Dispute ${dispute.id.slice(0, 8)}`}>
      <dl className="mb-6 grid grid-cols-2 gap-x-6 gap-y-2 text-sm md:grid-cols-4">
        <Fact label="State" value={dispute.state} mono />
        <Fact label="Regime" value={dispute.regime} mono />
        <Fact label="Reason" value={dispute.reason} mono />
        <Fact label="Amount" value={formatMoney(dispute.disputedAmount, dispute.currency)} />
        <Fact label="Appeals used" value={String(dispute.appeals)} />
      </dl>

      {dispute.risk && (
        <section className="mb-8">
          <h2 className="mb-2 text-lg font-medium">Fraud risk</h2>
          <RiskPanel risk={dispute.risk} />
        </section>
      )}

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Regulatory clocks</h2>
        <Deadlines deadlines={dispute.deadlines} />
      </section>

      {dispute.questionnaire && (
        <section className="mb-8">
          <h2 className="mb-2 text-lg font-medium">Questionnaire</h2>
          <QuestionnairePanel
            questionnaire={dispute.questionnaire}
            canReceive={dispute.allowedEvents.includes('RECEIVE_QUESTIONNAIRE')}
            fields={fieldErrors}
            busy={applyEventMutation.isPending}
            onReceive={(answers) => applyWithFormFacts('RECEIVE_QUESTIONNAIRE', { answers })}
          />
        </section>
      )}

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Actions</h2>
        {dispute.allowedEvents.length === 0 ? (
          <p className="text-sm text-neutral-600">This dispute is closed; nothing more can happen to it.</p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {dispute.allowedEvents
              .filter((event) => event !== 'RECEIVE_QUESTIONNAIRE')
              .map((event) => (
                <Button
                  key={event}
                  variant="action"
                  size="sm"
                  disabled={applyEventMutation.isPending}
                  onClick={() => applyWithFormFacts(event)}
                >
                  {eventInFlight === event && applyEventMutation.isPending ? '...' : event}
                </Button>
              ))}
          </div>
        )}
        {(canRefund || canSettle) && (
          <div className="mt-3 flex flex-wrap gap-6 text-sm">
            {canRefund && (
              <label className="block">
                Customer liability ({dispute.currency}, optional)
                <input
                  value={liability}
                  onChange={(event) => setLiability(event.target.value)}
                  inputMode="decimal"
                  placeholder="0.00"
                  className={cn(
                    inputVariants({ invalid: Boolean(fieldErrors.liability), mono: true }),
                    'w-40',
                  )}
                  {...invalidProps('liability', fieldErrors.liability)}
                />
                <FieldError id="liability-error" message={fieldErrors.liability} />
              </label>
            )}
            {canRefund && dispute.risk?.tier === 'HIGH' && (
              <label className="block basis-full">
                Justification for crediting despite the HIGH risk score
                <textarea
                  value={riskOverride}
                  onChange={(event) => setRiskOverride(event.target.value)}
                  rows={2}
                  className={cn(inputVariants({ invalid: Boolean(fieldErrors.riskOverride) }), 'max-w-xl')}
                  {...invalidProps('riskOverride', fieldErrors.riskOverride)}
                />
                <FieldError id="riskOverride-error" message={fieldErrors.riskOverride} />
              </label>
            )}
            {canSettle && (
              <label className="block">
                Outstanding advance on close
                <select
                  value={settlement}
                  onChange={(event) => setSettlement(event.target.value)}
                  className={cn(inputVariants({ invalid: Boolean(fieldErrors.settlement) }), 'w-auto')}
                  {...invalidProps('settlement', fieldErrors.settlement)}
                >
                  <option value="">regime default</option>
                  <option value="RECOVERED">recovered</option>
                  <option value="WRITTEN_OFF">written off</option>
                </select>
                <FieldError id="settlement-error" message={fieldErrors.settlement} />
              </label>
            )}
          </div>
        )}
        {applyFailure && !failureShownAtField && (
          <div className="mt-4">
            <FailureBanner failure={applyFailure} onRetry={applyEventMutation.clearFailure} />
          </div>
        )}
      </section>

      <section className="mb-8">
        <h2 className="mb-2 text-lg font-medium">Ledger</h2>
        <Ledger ledger={dispute.ledger} balances={dispute.balances} currency={dispute.currency} />
      </section>

      <section className="mb-8">
        <div className="mb-2 flex items-baseline justify-between">
          <h2 className="text-lg font-medium">Communications</h2>
          <Link
            to="/$tenant/disputes/$disputeId/communications"
            params={{ tenant, disputeId: dispute.id }}
            className="text-sm underline"
          >
            Open the communications panel
          </Link>
        </div>
        <Notices notices={dispute.notices} tenant={tenant} disputeId={dispute.id} />
      </section>

      <section>
        <h2 className="mb-2 text-lg font-medium">Event log</h2>
        <EventLog events={dispute.events} />
      </section>
    </AppShell>
  )
}

const Fact = ({ label, value, mono = false }: { label: string; value: string; mono?: boolean }) => (
  <div>
    <dt className="text-neutral-500">{label}</dt>
    <dd className={cn(mono && 'font-mono')}>{value}</dd>
  </div>
)

export const Route = createFileRoute('/$tenant/disputes/$disputeId')({
  loader: ({ params, context }) =>
    context.queryClient.query({ ...disputeQuery(params.disputeId), staleTime: 'static' }),
  component: DisputePage,
})
