import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import type { Static } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'

import {
  CreateDisputeRequest,
  DisputeReason as DisputeReasonSchema,
  DisputeState as DisputeStateSchema,
} from '#/api/schemas.gen'
import type { DisputeState } from '#/api/views'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { parsed, schemaValidator } from '#/forms/schema'
import { formatMoney } from '#/lib/money'
import { disputeQuery, disputesQuery, type DisputeListSearch } from '#/queries/disputes'
import { useServerMutation } from '#/queries/use-server-mutation'
import { openDispute } from '#/server/functions/disputes'

// from the generated schema, so the filter cannot offer a state the API lacks
const DISPUTE_STATES = DisputeStateSchema.anyOf.map((literal) => ({
  value: literal.const,
  label: literal.const,
}))
const DISPUTE_REASONS = DisputeReasonSchema.anyOf.map((literal) => ({
  value: literal.const,
  label: literal.const,
}))
const isDisputeState = (value: unknown): value is DisputeState =>
  typeof value === 'string' && Value.Check(DisputeStateSchema, value)

const Workbench = () => {
  const { tenant } = Route.useParams()
  const search = Route.useSearch()
  const { state, cursor, overdue } = search
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: loadedPage } = useSuspenseQuery(disputesQuery(search))
  const activeFilters = { ...(state ? { state } : {}), ...(overdue ? { overdue: true } : {}) }
  const navigate = useNavigate()
  const openDisputeMutation = useServerMutation(
    (request: Static<typeof CreateDisputeRequest>) => openDispute({ data: { ...request, actor: 'analyst' } }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: (openedDispute) =>
        navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: openedDispute.id } }),
    },
  )
  const openForm = useAppForm({
    defaultValues: { transactionId: '', reason: 'UNAUTHORISED' },
    validators: { onSubmit: schemaValidator(CreateDisputeRequest) },
    onSubmit: ({ value, formApi }) =>
      submitTo(formApi, () => openDisputeMutation.mutateAsync(parsed(CreateDisputeRequest, value))),
  })
  const filterForm = useAppForm({
    defaultValues: { state: state ?? '', overdue: overdue ?? false },
    onSubmit: ({ value }) =>
      navigate({
        to: '/$tenant',
        params: { tenant },
        search: {
          ...(isDisputeState(value.state) ? { state: value.state } : {}),
          ...(value.overdue ? { overdue: true } : {}),
        },
      }),
  })
  const findForm = useAppForm({
    defaultValues: { disputeId: '' },
    onSubmit: ({ value }) => {
      const disputeId = value.disputeId.trim()
      if (disputeId) return navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId } })
      return undefined
    },
  })
  const openFailure = openDisputeMutation.failure
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  return (
    <AppShell title="Disputes">
      <section className="mb-10">
        <div className="mb-3 flex items-baseline justify-between">
          <h2 className="text-lg font-medium">Recent disputes</h2>
          <form className="flex items-end gap-2 text-sm" onSubmit={submitting(filterForm)}>
            <filterForm.AppField name="state">
              {(field) => (
                <field.SelectField
                  label="State"
                  options={DISPUTE_STATES}
                  placeholder="any"
                  className="text-neutral-600"
                  inputClassName="mt-0 w-auto"
                />
              )}
            </filterForm.AppField>
            <filterForm.AppField name="overdue">
              {(field) => <field.CheckboxField label="overdue only" className="mb-1 text-neutral-600" />}
            </filterForm.AppField>
            <filterForm.AppForm>
              <filterForm.SubmitButton variant="secondary" size="xs">
                Filter
              </filterForm.SubmitButton>
            </filterForm.AppForm>
          </form>
        </div>
        {loadedPage.failure ? (
          <FailureBanner failure={loadedPage.failure} />
        ) : loadedPage.value.items.length > 0 ? (
          <>
            <table className="w-full text-left text-sm">
              <thead className="text-neutral-500">
                <tr>
                  <th className="py-1 pr-4 font-normal">Dispute</th>
                  <th className="py-1 pr-4 font-normal">State</th>
                  <th className="py-1 pr-4 font-normal">Regime</th>
                  <th className="py-1 pr-4 font-normal">Amount</th>
                  <th className="py-1 pr-4 font-normal">Risk</th>
                  <th className="py-1 pr-4 font-normal">Next clock</th>
                  <th className="py-1 font-normal">Opened</th>
                </tr>
              </thead>
              <tbody>
                {loadedPage.value.items.map((dispute) => (
                  <tr key={dispute.id} className="border-t border-neutral-200">
                    <td className="py-1 pr-4 font-mono">
                      <Link
                        to="/$tenant/disputes/$disputeId"
                        params={{ tenant, disputeId: dispute.id }}
                        className="underline"
                      >
                        {dispute.id.slice(0, 8)}
                      </Link>
                    </td>
                    <td className="py-1 pr-4 font-mono">{dispute.state}</td>
                    <td className="py-1 pr-4 font-mono">{dispute.regime}</td>
                    <td className="py-1 pr-4">{formatMoney(dispute.disputedAmount, dispute.currency)}</td>
                    <td className="py-1 pr-4">
                      {dispute.riskTier ? (
                        <RiskBadge tier={dispute.riskTier} score={dispute.riskScore} />
                      ) : (
                        <span className="text-neutral-400">none</span>
                      )}
                    </td>
                    <td className="py-1 pr-4">
                      {dispute.nextDeadline ? (
                        <span className="flex items-center gap-2">
                          <DeadlineBadge status={dispute.nextDeadline.status} />
                          <span className="text-neutral-600">
                            {daysRemaining(dispute.nextDeadline.dueAt)}
                          </span>
                        </span>
                      ) : (
                        <span className="text-neutral-400">none</span>
                      )}
                    </td>
                    <td className="py-1 text-neutral-500">
                      {dispute.openedAt.slice(0, 16).replace('T', ' ')}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="mt-3 flex gap-3 text-sm">
              {cursor && (
                <Link to="/$tenant" params={{ tenant }} search={activeFilters} className="underline">
                  Newest
                </Link>
              )}
              {loadedPage.value.nextCursor && (
                <Link
                  to="/$tenant"
                  params={{ tenant }}
                  search={{ ...activeFilters, cursor: loadedPage.value.nextCursor }}
                  className="underline"
                >
                  Older
                </Link>
              )}
            </div>
          </>
        ) : (
          <p className="text-sm text-neutral-600">
            No {overdue ? 'overdue ' : ''}disputes{state ? ` in ${state}` : ''}
            {overdue ? '.' : ' yet.'}
          </p>
        )}
      </section>

      <div className="grid gap-8 md:grid-cols-2">
        <section>
          <h2 className="mb-3 text-lg font-medium">Open a dispute</h2>
          <form className="space-y-3" onSubmit={submitting(openForm)}>
            <openForm.AppField name="transactionId">
              {(field) => (
                <field.TextField
                  label="Transaction ID"
                  mono
                  inputClassName="px-3 py-2"
                  placeholder="00000000-0000-8000-8000-000000000101"
                />
              )}
            </openForm.AppField>
            <openForm.AppField name="reason">
              {(field) => (
                <field.SelectField label="Reason" options={DISPUTE_REASONS} mono inputClassName="px-3 py-2" />
              )}
            </openForm.AppField>
            <openForm.AppForm>
              <openForm.SubmitButton busy={openDisputeMutation.isPending}>Open</openForm.SubmitButton>
            </openForm.AppForm>
          </form>
          {openFailure && openFailure.kind !== 'validation' && (
            <div className="mt-4">
              <FailureBanner failure={openFailure} />
            </div>
          )}
        </section>
        <section>
          <h2 className="mb-3 text-lg font-medium">Find a dispute</h2>
          <form className="space-y-3" onSubmit={submitting(findForm)}>
            <findForm.AppField name="disputeId">
              {(field) => <field.TextField label="Dispute ID" mono inputClassName="px-3 py-2" />}
            </findForm.AppField>
            <findForm.AppForm>
              <findForm.SubmitButton>Show</findForm.SubmitButton>
            </findForm.AppForm>
          </form>
        </section>
      </div>
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/')({
  validateSearch: (rawSearch: Record<string, unknown>): DisputeListSearch => ({
    ...(isDisputeState(rawSearch.state) ? { state: rawSearch.state } : {}),
    ...(typeof rawSearch.cursor === 'string' ? { cursor: rawSearch.cursor } : {}),
    ...(rawSearch.overdue === true || rawSearch.overdue === 'true' ? { overdue: true } : {}),
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps, context }) => context.queryClient.query({ ...disputesQuery(deps), staleTime: 'static' }),
  component: Workbench,
})
