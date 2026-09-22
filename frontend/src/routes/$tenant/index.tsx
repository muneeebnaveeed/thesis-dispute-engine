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
import { DisputeTable } from '#/components/disputes/dispute-table'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { Panel } from '#/components/ui/panel'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { parsed, schemaValidator } from '#/forms/schema'
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
  const openFailure = openDisputeMutation.failure
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  return (
    <AppShell title="Disputes">
      <Panel
        className="mb-6"
        bodyClassName="p-0"
        title="Recent disputes"
        actions={
          <form className="flex items-end gap-2 text-[13px]" onSubmit={submitting(filterForm)}>
            <filterForm.AppField name="state">
              {(field) => (
                <field.SelectField
                  label="State"
                  options={DISPUTE_STATES}
                  placeholder="any"
                  className="text-muted-foreground"
                  inputClassName="mt-0 w-auto"
                />
              )}
            </filterForm.AppField>
            <filterForm.AppField name="overdue">
              {(field) => <field.CheckboxField label="overdue only" className="mb-1 text-muted-foreground" />}
            </filterForm.AppField>
            <filterForm.AppForm>
              <filterForm.SubmitButton variant="secondary" size="xs">
                Filter
              </filterForm.SubmitButton>
            </filterForm.AppForm>
          </form>
        }
      >
        {loadedPage.failure ? (
          <div className="p-4">
            <FailureBanner failure={loadedPage.failure} />
          </div>
        ) : loadedPage.value.items.length > 0 ? (
          <>
            <DisputeTable disputes={loadedPage.value.items} tenant={tenant} />
            <div className="flex gap-3 border-t border-border bg-muted px-4 py-2 text-[13px]">
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
          <p className="px-4 py-3 text-[13px] text-muted-foreground">
            No {overdue ? 'overdue ' : ''}disputes{state ? ` in ${state}` : ''}
            {overdue ? '.' : ' yet.'}
          </p>
        )}
      </Panel>

      <div className="grid gap-6 md:grid-cols-2">
        <Panel title="Open a dispute">
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
        </Panel>
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
