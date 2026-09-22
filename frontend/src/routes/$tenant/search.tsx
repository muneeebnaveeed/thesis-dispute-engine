import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { Value } from '@sinclair/typebox/value'

import { DisputeReason as DisputeReasonSchema, DisputeState as DisputeStateSchema } from '#/api/schemas.gen'
import type { components } from '#/api/schema.gen'
import { DisputeTable } from '#/components/disputes/dispute-table'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { submitting, useAppForm } from '#/forms/app-form'
import { disputesQuery, type DisputeListSearch } from '#/queries/disputes'

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
  const { state, reason } = Route.useSearch()
  const { viewer } = useRouteContext({ from: '__root__' })
  const navigate = useNavigate()
  const filters: DisputeListSearch = { ...(state ? { state } : {}), ...(reason ? { reason } : {}) }
  const { data: loaded } = useQuery(disputesQuery(filters))

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
      <form className="mb-6 flex flex-wrap items-end gap-3" onSubmit={submitting(form)}>
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
          {(field) => <field.SelectField label="State" options={STATES} inputClassName="mt-0 w-auto" />}
        </form.AppField>
        <form.AppField name="reason">
          {(field) => <field.SelectField label="Reason" options={REASONS} inputClassName="mt-0 w-auto" />}
        </form.AppField>
        <form.AppForm>
          <form.SubmitButton variant="secondary">Search</form.SubmitButton>
        </form.AppForm>
      </form>

      {loaded?.failure ? (
        <FailureBanner failure={loaded.failure} />
      ) : loaded && loaded.value.items.length > 0 ? (
        <DisputeTable disputes={loaded.value.items} tenant={tenant} />
      ) : (
        <p className="text-sm text-muted-foreground">
          {loaded ? 'No dispute matches those filters.' : 'Searching...'}
        </p>
      )}
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/search')({
  validateSearch: (raw: Record<string, unknown>): { state?: DisputeState; reason?: DisputeReason } => ({
    ...(isState(raw.state) ? { state: raw.state } : {}),
    ...(isReason(raw.reason) ? { reason: raw.reason } : {}),
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps, context }) => context.queryClient.query({ ...disputesQuery(deps), staleTime: 'static' }),
  component: SearchPage,
})
