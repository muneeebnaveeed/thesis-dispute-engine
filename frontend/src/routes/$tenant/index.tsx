import { useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import type { Static } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import { useState } from 'react'

import { classify } from '#/api/failure'
import {
  CreateDisputeRequest,
  DisputeReason as DisputeReasonSchema,
  DisputeState as DisputeStateSchema,
} from '#/api/schemas.gen'
import type { DisputeState } from '#/api/views'
import { DeadlineBadge, daysRemaining } from '#/components/disputes/deadlines'
import { RiskBadge } from '#/components/disputes/risk'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner, FieldError } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'
import { serverFieldErrors, validateForm } from '#/forms/validate-form'
import { cn } from '#/lib/cn'
import { formatMoney } from '#/lib/money'
import { disputeQuery, disputesQuery, type DisputeListSearch } from '#/queries/disputes'
import { fieldErrorsOf, useServerMutation } from '#/queries/use-server-mutation'
import { openDispute } from '#/server/functions/disputes'

// from the generated schema, so the filter cannot offer a state the API lacks
const DISPUTE_STATES = DisputeStateSchema.anyOf.map((literal) => literal.const)
const DISPUTE_REASONS = DisputeReasonSchema.anyOf.map((literal) => literal.const)
const isDisputeState = (value: unknown): value is DisputeState =>
  typeof value === 'string' && Value.Check(DisputeStateSchema, value)

const Workbench = () => {
  const { tenant } = Route.useParams()
  const search = Route.useSearch()
  const { state, cursor, overdue } = search
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: disputePage } = useSuspenseQuery(disputesQuery(search))
  const activeFilters = { ...(state ? { state } : {}), ...(overdue ? { overdue: true } : {}) }
  const navigate = useNavigate()
  const [clientFieldErrors, setClientFieldErrors] = useState<Record<string, string>>({})
  const openDisputeMutation = useServerMutation(
    (request: Static<typeof CreateDisputeRequest>) => openDispute({ data: request }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: (openedDispute) =>
        navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: openedDispute.id } }),
    },
  )
  const openFailure = openDisputeMutation.failure
  const fieldErrors = {
    ...clientFieldErrors,
    ...fieldErrorsOf(openFailure),
    ...(openFailure?.kind === 'validation' ? serverFieldErrors(openFailure.problem) : {}),
  }
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  const openFromForm = (form: FormData) => {
    const validated = validateForm(CreateDisputeRequest, form)
    setClientFieldErrors(validated.fields)
    if (validated.value) openDisputeMutation.mutate({ ...validated.value, actor: 'analyst' })
  }

  return (
    <AppShell title="Disputes">
      <section className="mb-10">
        <div className="mb-3 flex items-baseline justify-between">
          <h2 className="text-lg font-medium">Recent disputes</h2>
          <form
            className="flex items-center gap-2 text-sm"
            onSubmit={(event) => {
              event.preventDefault()
              const filterForm = new FormData(event.currentTarget)
              const chosenState = filterForm.get('state')
              void navigate({
                to: '/$tenant',
                params: { tenant },
                search: {
                  ...(isDisputeState(chosenState) ? { state: chosenState } : {}),
                  ...(filterForm.get('overdue') ? { overdue: true } : {}),
                },
              })
            }}
          >
            <label htmlFor="state-filter" className="text-neutral-600">
              State
            </label>
            <select
              id="state-filter"
              name="state"
              defaultValue={state ?? ''}
              className={cn(inputVariants(), 'mt-0 w-auto')}
            >
              <option value="">any</option>
              {DISPUTE_STATES.map((disputeState) => (
                <option key={disputeState} value={disputeState}>
                  {disputeState}
                </option>
              ))}
            </select>
            <label className="flex items-center gap-1 text-neutral-600">
              <input type="checkbox" name="overdue" defaultChecked={overdue ?? false} />
              overdue only
            </label>
            <Button type="submit" variant="secondary" size="xs">
              Filter
            </Button>
          </form>
        </div>
        {disputePage.problem ? (
          <FailureBanner
            failure={
              classify({ error: disputePage.problem }) ?? {
                kind: 'unexpected',
                status: 0,
                message: 'no data',
              }
            }
          />
        ) : disputePage.value && disputePage.value.items.length > 0 ? (
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
                {disputePage.value.items.map((dispute) => (
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
              {disputePage.value.nextCursor && (
                <Link
                  to="/$tenant"
                  params={{ tenant }}
                  search={{ ...activeFilters, cursor: disputePage.value.nextCursor }}
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
          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault()
              openFromForm(new FormData(event.currentTarget))
            }}
          >
            <label className="block text-sm">
              Transaction ID
              <input
                name="transactionId"
                className={cn(
                  inputVariants({ invalid: Boolean(fieldErrors.transactionId), mono: true }),
                  'px-3 py-2',
                )}
                placeholder="00000000-0000-8000-8000-000000000101"
                {...invalidProps('transactionId', fieldErrors.transactionId)}
              />
            </label>
            <FieldError id="transactionId-error" message={fieldErrors.transactionId} />
            <label className="block text-sm">
              Reason
              <select
                name="reason"
                defaultValue="UNAUTHORISED"
                className={cn(inputVariants({ mono: true }), 'px-3 py-2')}
              >
                {DISPUTE_REASONS.map((reason) => (
                  <option key={reason} value={reason}>
                    {reason}
                  </option>
                ))}
              </select>
            </label>
            <FieldError id="reason-error" message={fieldErrors.reason} />
            <Button type="submit" disabled={openDisputeMutation.isPending}>
              Open
            </Button>
          </form>
          {openFailure && (openFailure.kind !== 'validation' || Object.keys(fieldErrors).length === 0) && (
            <div className="mt-4">
              <FailureBanner failure={openFailure} />
            </div>
          )}
        </section>
        <section>
          <h2 className="mb-3 text-lg font-medium">Find a dispute</h2>
          <form
            className="space-y-3"
            onSubmit={(event) => {
              event.preventDefault()
              const typed = new FormData(event.currentTarget).get('disputeId')
              const disputeId = typeof typed === 'string' ? typed.trim() : ''
              if (disputeId)
                void navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId } })
            }}
          >
            <label className="block text-sm">
              Dispute ID
              <input name="disputeId" className={cn(inputVariants({ mono: true }), 'px-3 py-2')} />
            </label>
            <Button type="submit">Show</Button>
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
