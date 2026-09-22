import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouteContext } from '@tanstack/react-router'

import type { Dispute } from '#/api/views'
import { Deadlines } from '#/components/disputes/deadlines'
import { EventLog } from '#/components/disputes/event-log'
import { Ledger } from '#/components/disputes/ledger'
import { Notices } from '#/components/disputes/notices'
import { QuestionnairePanel } from '#/components/disputes/questionnaire'
import { RiskPanel } from '#/components/disputes/risk'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Button } from '#/components/ui/button'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { cn } from '#/lib/utils'
import { formatMoney } from '#/lib/money'
import { disputeQuery } from '#/queries/disputes'
import { useServerMutation } from '#/queries/use-server-mutation'
import { applyEvent } from '#/server/functions/disputes'

type DisputeEvent = Dispute['allowedEvents'][number]

type ActionFacts = { liability: string; riskOverride: string; settlement: string }
const NO_FACTS: ActionFacts = { liability: '', riskOverride: '', settlement: '' }

const DisputePage = () => {
  const { tenant, disputeId } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: loadedDispute } = useSuspenseQuery(disputeQuery(disputeId))
  const queryClient = useQueryClient()
  const refreshDispute = () =>
    void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })

  const applyEventMutation = useServerMutation(
    ({ event, payload }: { event: DisputeEvent; payload: Record<string, unknown> }) =>
      applyEvent({ data: { disputeId, body: { event, actor: 'analyst', payload } } }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: () => actionForm.reset(),
    },
  )
  // the facts an action may carry; which ones travel depends on the button pressed, so the event is submit meta
  const actionForm = useAppForm({
    defaultValues: NO_FACTS,
    onSubmitMeta: { event: 'CLOSE' as DisputeEvent, payload: {} as Record<string, unknown> },
    onSubmit: async ({ value, meta: { event, payload }, formApi }) => {
      const facts: Record<string, unknown> = { ...payload }
      if (event === 'ISSUE_REFUND' && value.liability.trim()) facts.liability = value.liability.trim()
      if (event === 'ISSUE_REFUND' && value.riskOverride.trim())
        facts.riskOverride = value.riskOverride.trim()
      if (event === 'CLOSE' && value.settlement) facts.settlement = value.settlement
      await submitTo(formApi, () => applyEventMutation.mutateAsync({ event, payload: facts }))
      // conflict: the state moved under us, show the truth next to the message
      if (applyEventMutation.failure?.kind === 'conflict') refreshDispute()
    },
  })
  const applyFailure = applyEventMutation.failure

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
  const apply = (event: DisputeEvent, payload: Record<string, unknown> = {}) =>
    void actionForm.handleSubmit({ event, payload })

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
            busy={applyEventMutation.isPending}
            onReceive={(answers) =>
              applyEventMutation.mutateAsync({ event: 'RECEIVE_QUESTIONNAIRE', payload: { answers } })
            }
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
                  onClick={() => apply(event)}
                >
                  {applyEventMutation.isPending && applyEventMutation.variables?.event === event
                    ? '...'
                    : event}
                </Button>
              ))}
          </div>
        )}
        {(canRefund || canSettle) && (
          <form
            className="mt-3 flex flex-wrap gap-6 text-sm"
            onSubmit={submitting({
              handleSubmit: () =>
                actionForm.handleSubmit({ event: canRefund ? 'ISSUE_REFUND' : 'CLOSE', payload: {} }),
            })}
          >
            {canRefund && (
              <actionForm.AppField name="liability">
                {(field) => (
                  <field.TextField
                    label={`Customer liability (${dispute.currency}, optional)`}
                    inputMode="decimal"
                    placeholder="0.00"
                    mono
                    inputClassName="w-40"
                  />
                )}
              </actionForm.AppField>
            )}
            {canRefund && dispute.risk?.tier === 'HIGH' && (
              <actionForm.AppField name="riskOverride">
                {(field) => (
                  <field.TextareaField
                    label="Justification for crediting despite the HIGH risk score"
                    rows={2}
                    className="basis-full"
                    inputClassName="max-w-xl"
                  />
                )}
              </actionForm.AppField>
            )}
            {canSettle && (
              <actionForm.AppField name="settlement">
                {(field) => (
                  <field.SelectField
                    label="Outstanding advance on close"
                    placeholder="regime default"
                    options={[
                      { value: 'RECOVERED', label: 'recovered' },
                      { value: 'WRITTEN_OFF', label: 'written off' },
                    ]}
                    inputClassName="w-auto"
                  />
                )}
              </actionForm.AppField>
            )}
          </form>
        )}
        {applyFailure && applyFailure.kind !== 'validation' && (
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
