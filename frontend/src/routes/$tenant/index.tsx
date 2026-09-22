import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'
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
import { Button } from '#/components/ui/button'
import { GridPager, GridToolbar, ToolbarFill, ToolbarSeparator } from '#/components/ui/grid'
import { Panel } from '#/components/ui/panel'
import { Window } from '#/components/ui/window'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { parsed, schemaValidator } from '#/forms/schema'
import { disputeQuery, disputesQuery, type DisputeListSearch } from '#/queries/disputes'
import { useDisputeReasonSuggestion } from '#/queries/suggestions'
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

const DEFAULT_PAGE_SIZE = 25

const Workbench = () => {
  const { tenant } = Route.useParams()
  const search = Route.useSearch()
  const { state, overdue, limit } = search
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: loadedPage } = useSuspenseQuery(disputesQuery(search))
  const activeFilters = {
    ...(state ? { state } : {}),
    ...(overdue ? { overdue: true } : {}),
    ...(limit ? { limit } : {}),
  }
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  // keyset paging knows no page numbers, so the grid remembers the cursors it walked in to step back
  const [walked, setWalked] = useState<string[]>([])
  const [opening, setOpening] = useState(false)
  const pageSize = limit ?? DEFAULT_PAGE_SIZE
  const goToPage = (cursor: string | undefined, trail: string[]) => {
    setWalked(trail)
    void navigate({
      to: '/$tenant',
      params: { tenant },
      search: { ...activeFilters, ...(cursor ? { cursor } : {}) },
    })
  }
  const openDisputeMutation = useServerMutation(
    (request: Static<typeof CreateDisputeRequest>) => openDispute({ data: { ...request, actor: 'analyst' } }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: (openedDispute) => {
        setOpening(false)
        return navigate({
          to: '/$tenant/disputes/$disputeId',
          params: { tenant, disputeId: openedDispute.id },
        })
      },
    },
  )
  // the reason the model offered, cleared the moment the analyst touches the dropdown: what is opened is
  // always what the dropdown says, and this only records whether a machine put it there
  const [proposed, setProposed] = useState<{ reason: string; probability: number } | null>(null)
  const reasonSuggestion = useDisputeReasonSuggestion()
  const openForm = useAppForm({
    defaultValues: { transactionId: '', reason: 'UNAUTHORISED', description: '' },
    validators: { onSubmit: schemaValidator(CreateDisputeRequest) },
    onSubmit: ({ value, formApi }) =>
      submitTo(formApi, () =>
        openDisputeMutation.mutateAsync(
          parsed(CreateDisputeRequest, {
            transactionId: value.transactionId,
            reason: value.reason,
            // what the analyst was shown travels with what they chose, so acceptance can be counted
            ...(proposed ? { suggestion: proposed } : {}),
          }),
        ),
      ),
  })
  const readDescription = async (description: string) => {
    const trimmed = description.trim()
    if (!trimmed) return
    const proposal = await reasonSuggestion.mutateAsync(trimmed).catch(() => null)
    if (!proposal?.reason || proposal.probability === undefined) return
    openForm.setFieldValue('reason', proposal.reason)
    setProposed({ reason: proposal.reason, probability: proposal.probability })
  }
  const filterForm = useAppForm({
    defaultValues: { state: state ?? '', overdue: overdue ?? false },
    onSubmit: ({ value }) => {
      setWalked([])
      return navigate({
        to: '/$tenant',
        params: { tenant },
        search: {
          ...(isDisputeState(value.state) ? { state: value.state } : {}),
          ...(value.overdue ? { overdue: true } : {}),
          ...(limit ? { limit } : {}),
        },
      })
    },
  })
  const openFailure = openDisputeMutation.failure
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  return (
    <AppShell title="Disputes">
      <Panel bodyClassName="p-0" title="Recent disputes">
        <GridToolbar>
          <Button variant="secondary" size="sm" onClick={() => setOpening(true)}>
            New dispute
          </Button>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void queryClient.invalidateQueries({ queryKey: disputeQuery(null).queryKey })}
          >
            Refresh
          </Button>
          <ToolbarSeparator />
          <form className="flex items-center gap-1.5" onSubmit={submitting(filterForm)}>
            <filterForm.AppField name="state">
              {(field) => (
                <field.SelectField
                  label="State"
                  options={DISPUTE_STATES}
                  placeholder="any"
                  className="flex items-center gap-1 [&_.field-control]:contents"
                  inputClassName="mt-0 w-auto"
                />
              )}
            </filterForm.AppField>
            <filterForm.AppField name="overdue">
              {(field) => <field.CheckboxField label="overdue only" />}
            </filterForm.AppField>
            <filterForm.AppForm>
              <filterForm.SubmitButton variant="secondary" size="sm">
                Filter
              </filterForm.SubmitButton>
            </filterForm.AppForm>
          </form>
          <ToolbarFill />
        </GridToolbar>
        {loadedPage.failure ? (
          <div className="p-2">
            <FailureBanner failure={loadedPage.failure} />
          </div>
        ) : loadedPage.value.items.length > 0 ? (
          <DisputeTable disputes={loadedPage.value.items} tenant={tenant} />
        ) : (
          <p className="px-2 py-1.5 text-[11px] text-muted-foreground">
            No {overdue ? 'overdue ' : ''}disputes{state ? ` in ${state}` : ''}
            {overdue ? '.' : ' yet.'}
          </p>
        )}
        <GridPager
          noun="disputes"
          page={walked.length + 1}
          pageSize={pageSize}
          shown={loadedPage.value?.items.length ?? 0}
          hasNext={Boolean(loadedPage.value?.nextCursor)}
          onFirst={() => goToPage(undefined, [])}
          onPrevious={() => goToPage(walked.at(-1), walked.slice(0, -1))}
          onNext={() => goToPage(loadedPage.value?.nextCursor, [...walked, search.cursor ?? ''])}
          onPageSize={(size) => {
            setWalked([])
            void navigate({
              to: '/$tenant',
              params: { tenant },
              search: { ...activeFilters, limit: size },
            })
          }}
        />
      </Panel>

      <Window
        open={opening}
        onOpenChange={setOpening}
        title="Open a dispute"
        footer={
          <openForm.AppForm>
            <openForm.SubmitButton busy={openDisputeMutation.isPending} form="open-dispute">
              Open
            </openForm.SubmitButton>
            <Button variant="secondary" onClick={() => setOpening(false)}>
              Cancel
            </Button>
          </openForm.AppForm>
        }
      >
        <form id="open-dispute" className="form-rows" onSubmit={submitting(openForm)}>
          <openForm.AppField name="description">
            {(field) => (
              <field.TextareaField
                label="What the customer said"
                hint="(optional, read to fill in the reason)"
                rows={3}
                onBlur={(event) => void readDescription(event.target.value)}
              />
            )}
          </openForm.AppField>
          <openForm.AppField name="transactionId">
            {(field) => (
              <field.TextField
                label="Transaction ID"
                mono
                placeholder="00000000-0000-8000-8000-000000000101"
              />
            )}
          </openForm.AppField>
          <openForm.AppField name="reason">
            {(field) => (
              <field.SelectField
                label="Reason"
                options={DISPUTE_REASONS}
                mono
                onChange={() => setProposed(null)}
              />
            )}
          </openForm.AppField>
          {proposed && (
            <p className="text-[11px] text-muted-foreground">
              Reason suggested from the description ({Math.round(proposed.probability * 100)}% sure). Change
              it if it is wrong.
            </p>
          )}
        </form>
        {openFailure && openFailure.kind !== 'validation' && (
          <div className="mt-2">
            <FailureBanner failure={openFailure} />
          </div>
        )}
      </Window>
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/')({
  validateSearch: (rawSearch: Record<string, unknown>): DisputeListSearch => ({
    ...(isDisputeState(rawSearch.state) ? { state: rawSearch.state } : {}),
    ...(typeof rawSearch.cursor === 'string' ? { cursor: rawSearch.cursor } : {}),
    ...(typeof rawSearch.limit === 'number' ? { limit: rawSearch.limit } : {}),
    ...(rawSearch.overdue === true || rawSearch.overdue === 'true' ? { overdue: true } : {}),
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps, context }) => context.queryClient.query({ ...disputesQuery(deps), staleTime: 'static' }),
  component: Workbench,
})
