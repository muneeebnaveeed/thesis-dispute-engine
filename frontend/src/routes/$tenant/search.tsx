import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'
import { Value } from '@sinclair/typebox/value'

import { DisputeReason as DisputeReasonSchema, DisputeState as DisputeStateSchema } from '#/api/schemas.gen'
import type { components } from '#/api/schema.gen'
import { DisputeTable } from '#/components/disputes/dispute-table'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { GridPager, GridToolbar, ToolbarFill } from '#/components/ui/grid'
import { Panel } from '#/components/ui/panel'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { submitting, useAppForm } from '#/forms/app-form'
import { disputeQuery, disputesQuery, type DisputeListSearch } from '#/queries/disputes'

type DisputeState = components['schemas']['DisputeState']
type DisputeReason = components['schemas']['DisputeReason']

const ANY = { value: '', label: 'any' }
const STATES = [ANY, ...DisputeStateSchema.anyOf.map((l) => ({ value: l.const, label: l.const }))]
const REASONS = [ANY, ...DisputeReasonSchema.anyOf.map((l) => ({ value: l.const, label: l.const }))]
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i

const isState = (v: unknown): v is DisputeState => typeof v === 'string' && Value.Check(DisputeStateSchema, v)
const isReason = (v: unknown): v is DisputeReason =>
  typeof v === 'string' && Value.Check(DisputeReasonSchema, v)

const SearchPage = () => {
  const { tenant } = Route.useParams()
  const { state, reason, cursor, limit } = Route.useSearch()
  const { viewer } = useRouteContext({ from: '__root__' })
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [walked, setWalked] = useState<string[]>([])
  const pageSize = limit ?? 25
  const filters: DisputeListSearch = {
    ...(state ? { state } : {}),
    ...(reason ? { reason } : {}),
    ...(cursor ? { cursor } : {}),
    ...(limit ? { limit } : {}),
  }
  const { data: loaded } = useQuery(disputesQuery(filters))
  const goToPage = (nextCursor: string | undefined, trail: string[]) => {
    setWalked(trail)
    void navigate({
      to: '/$tenant/search',
      params: { tenant },
      search: {
        ...(state ? { state } : {}),
        ...(reason ? { reason } : {}),
        ...(limit ? { limit } : {}),
        ...(nextCursor ? { cursor: nextCursor } : {}),
      },
    })
  }

  const form = useAppForm({
    defaultValues: { disputeId: '', state: state ?? '', reason: reason ?? '' },
    onSubmit: async ({ value }) => {
      const disputeId = value.disputeId.trim()
      // an id is a jump, not a filter: the list endpoint does not search by id and the dispute may be on any page
      if (UUID.test(disputeId)) {
        await navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId } })
        return
      }
      await navigate({
        to: '/$tenant/search',
        params: { tenant },
        search: {
          ...(isState(value.state) ? { state: value.state } : {}),
          ...(isReason(value.reason) ? { reason: value.reason } : {}),
        },
      })
    },
  })

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  return (
    <AppShell title="Search">
      <Panel className="mb-2" title="Find a dispute">
        <form className="form-rows" onSubmit={submitting(form)}>
          <form.AppField name="disputeId">
            {(field) => (
              <field.TextField
                label="Dispute ID"
                hint="(paste one to open it)"
                mono
                inputClassName="w-[26rem] px-3 py-2"
                placeholder="00000000-0000-8000-8000-000000000101"
              />
            )}
          </form.AppField>
          <form.AppField name="state">
            {(field) => <field.SelectField label="State" options={STATES} />}
          </form.AppField>
          <form.AppField name="reason">
            {(field) => <field.SelectField label="Reason" options={REASONS} />}
          </form.AppField>
          <div className="mt-2 flex justify-end border-t border-border pt-2">
            <form.AppForm>
              <form.SubmitButton variant="secondary">Search</form.SubmitButton>
            </form.AppForm>
          </div>
        </form>
      </Panel>

      <Panel title="Results" bodyClassName="p-0">
        <GridToolbar>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => void queryClient.invalidateQueries({ queryKey: disputeQuery(null).queryKey })}
          >
            Refresh
          </Button>
          <ToolbarFill />
        </GridToolbar>
        {loaded?.failure ? (
          <div className="p-2">
            <FailureBanner failure={loaded.failure} />
          </div>
        ) : loaded && loaded.value.items.length > 0 ? (
          <DisputeTable disputes={loaded.value.items} tenant={tenant} />
        ) : (
          <p className="px-2 py-1.5 text-[11px] text-muted-foreground">
            {loaded ? 'No dispute matches those filters.' : 'Searching...'}
          </p>
        )}
        <GridPager
          noun="disputes"
          page={walked.length + 1}
          pageSize={pageSize}
          shown={loaded?.value?.items.length ?? 0}
          hasNext={Boolean(loaded?.value?.nextCursor)}
          onFirst={() => goToPage(undefined, [])}
          onPrevious={() => goToPage(walked.at(-1), walked.slice(0, -1))}
          onNext={() => goToPage(loaded?.value?.nextCursor, [...walked, cursor ?? ''])}
          onPageSize={(size) => {
            setWalked([])
            void navigate({
              to: '/$tenant/search',
              params: { tenant },
              search: {
                ...(state ? { state } : {}),
                ...(reason ? { reason } : {}),
                limit: size,
              },
            })
          }}
        />
      </Panel>
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/search')({
  validateSearch: (
    raw: Record<string, unknown>,
  ): { state?: DisputeState; reason?: DisputeReason; cursor?: string; limit?: number } => ({
    ...(isState(raw.state) ? { state: raw.state } : {}),
    ...(isReason(raw.reason) ? { reason: raw.reason } : {}),
    ...(typeof raw.cursor === 'string' ? { cursor: raw.cursor } : {}),
    ...(typeof raw.limit === 'number' ? { limit: raw.limit } : {}),
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps, context }) => context.queryClient.query({ ...disputesQuery(deps), staleTime: 'static' }),
  component: SearchPage,
})
