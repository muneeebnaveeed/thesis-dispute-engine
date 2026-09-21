import { Link, createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import { classify, type Failure } from '#/api/failure'
import { CreateDisputeRequest } from '#/api/schemas.gen'
import { AppShell } from '#/components/app-shell'
import { DeadlineBadge, remaining } from '#/components/deadlines'
import { RiskBadge } from '#/components/risk'
import { FailureBanner, FieldError } from '#/components/failure-banner'
import { TenantMismatch } from '#/components/tenant-mismatch'
import { serverFields, validateForm } from '#/forms/validate-form'
import { createDispute } from '#/server/disputes'
import { Value } from '@sinclair/typebox/value'

import { DisputeReason as DisputeReasonSchema, DisputeState as DisputeStateSchema } from '#/api/schemas.gen'
import { listDisputes, type DisputeState } from '#/server/disputes-list'

type Search = { state?: DisputeState; cursor?: string; overdue?: boolean }

// The workbench: open a dispute, find one, and the tenant's newest disputes with a state filter and paging.
export const Route = createFileRoute('/$tenant/')({
  validateSearch: (s: Record<string, unknown>): Search => ({
    ...(isState(s.state) ? { state: s.state } : {}),
    ...(typeof s.cursor === 'string' ? { cursor: s.cursor } : {}),
    ...(s.overdue === true || s.overdue === 'true' ? { overdue: true } : {}),
  }),
  loaderDeps: ({ search }) => search,
  loader: ({ deps }) => listDisputes({ data: deps }),
  component: Workbench,
})

// The generated schema is the source of truth for the enum, so the filter can never offer a state the API lacks.
const STATES = DisputeStateSchema.anyOf.map((l) => l.const)
const REASONS = DisputeReasonSchema.anyOf.map((l) => l.const)
const isState = (v: unknown): v is DisputeState => typeof v === 'string' && Value.Check(DisputeStateSchema, v)

const input =
  'w-full rounded-md border border-neutral-300 px-3 py-2 font-mono text-sm focus:border-neutral-500 focus:outline-none'
const button =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50'

function Workbench() {
  const { tenant } = Route.useParams()
  const page = Route.useLoaderData()
  const { state, cursor, overdue } = Route.useSearch()
  // The list's own filters, minus the cursor: what the paging and reset links carry along.
  const filters = { ...(state ? { state } : {}), ...(overdue ? { overdue: true } : {}) }
  const { viewer } = useRouteContext({ from: '__root__' })
  const navigate = useNavigate()
  const [failure, setFailure] = useState<Failure | null>(null)
  const [fields, setFields] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  // Validate with the contract's schema before any request; server-side field errors land in the same place.
  async function open(form: FormData) {
    setFailure(null)
    const checked = validateForm(CreateDisputeRequest, form)
    setFields(checked.fields)
    if (!checked.value) return
    setBusy(true)
    const res = await createDispute({ data: { ...checked.value, actor: 'analyst' } })
    setBusy(false)
    if (res.problem) {
      setFailure(classify({ error: res.problem }))
      setFields(serverFields(res.problem))
    } else if (res.value)
      void navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: res.value.id } })
  }

  return (
    <AppShell title="Disputes">
      <section className="mb-10">
        <div className="mb-3 flex items-baseline justify-between">
          <h2 className="text-lg font-medium">Recent disputes</h2>
          <form
            className="flex items-center gap-2 text-sm"
            onSubmit={(e) => {
              e.preventDefault()
              const form = new FormData(e.currentTarget)
              const raw = form.get('state')
              const next = isState(raw) ? raw : undefined
              void navigate({
                to: '/$tenant',
                params: { tenant },
                search: {
                  ...(next ? { state: next } : {}),
                  ...(form.get('overdue') ? { overdue: true } : {}),
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
              className="rounded-md border border-neutral-300 px-2 py-1"
            >
              <option value="">any</option>
              {STATES.map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
            <label className="flex items-center gap-1 text-neutral-600">
              <input type="checkbox" name="overdue" defaultChecked={overdue ?? false} />
              overdue only
            </label>
            <button
              type="submit"
              className="rounded-md border border-neutral-300 bg-white px-2 py-1 hover:bg-neutral-100"
            >
              Filter
            </button>
          </form>
        </div>
        {page.problem ? (
          <FailureBanner
            failure={
              classify({ error: page.problem }) ?? { kind: 'unexpected', status: 0, message: 'no data' }
            }
          />
        ) : page.value && page.value.items.length > 0 ? (
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
                {page.value.items.map((d) => (
                  <tr key={d.id} className="border-t border-neutral-200">
                    <td className="py-1 pr-4 font-mono">
                      <Link
                        to="/$tenant/disputes/$disputeId"
                        params={{ tenant, disputeId: d.id }}
                        className="underline"
                      >
                        {d.id.slice(0, 8)}
                      </Link>
                    </td>
                    <td className="py-1 pr-4 font-mono">{d.state}</td>
                    <td className="py-1 pr-4 font-mono">{d.regime}</td>
                    <td className="py-1 pr-4">
                      {d.disputedAmount} {d.currency}
                    </td>
                    <td className="py-1 pr-4">
                      {d.riskTier ? (
                        <RiskBadge tier={d.riskTier} score={d.riskScore} />
                      ) : (
                        <span className="text-neutral-400">none</span>
                      )}
                    </td>
                    <td className="py-1 pr-4">
                      {d.nextDeadline ? (
                        <span className="flex items-center gap-2">
                          <DeadlineBadge status={d.nextDeadline.status} />
                          <span className="text-neutral-600">{remaining(d.nextDeadline.dueAt)}</span>
                        </span>
                      ) : (
                        <span className="text-neutral-400">none</span>
                      )}
                    </td>
                    <td className="py-1 text-neutral-500">{d.openedAt.slice(0, 16).replace('T', ' ')}</td>
                  </tr>
                ))}
              </tbody>
            </table>
            <div className="mt-3 flex gap-3 text-sm">
              {cursor && (
                <Link to="/$tenant" params={{ tenant }} search={filters} className="underline">
                  Newest
                </Link>
              )}
              {page.value.nextCursor && (
                <Link
                  to="/$tenant"
                  params={{ tenant }}
                  search={{ ...filters, cursor: page.value.nextCursor }}
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
            onSubmit={(e) => {
              e.preventDefault()
              void open(new FormData(e.currentTarget))
            }}
          >
            <label className="block text-sm">
              Transaction ID
              <input
                name="transactionId"
                className={`${input} ${fields.transactionId ? 'border-red-400' : ''}`}
                placeholder="00000000-0000-8000-8000-000000000101"
                aria-invalid={fields.transactionId ? true : undefined}
                aria-describedby={fields.transactionId ? 'transactionId-error' : undefined}
              />
            </label>
            <FieldError id="transactionId-error" message={fields.transactionId} />
            <label className="block text-sm">
              Reason
              <select name="reason" defaultValue="UNAUTHORISED" className={input}>
                {REASONS.map((r) => (
                  <option key={r} value={r}>
                    {r}
                  </option>
                ))}
              </select>
            </label>
            <FieldError id="reason-error" message={fields.reason} />
            <button type="submit" className={button} disabled={busy}>
              Open
            </button>
          </form>
          {failure && (failure.kind !== 'validation' || Object.keys(fields).length === 0) && (
            <div className="mt-4">
              <FailureBanner failure={failure} />
            </div>
          )}
        </section>
        <section>
          <h2 className="mb-3 text-lg font-medium">Find a dispute</h2>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              const raw = new FormData(e.currentTarget).get('disputeId')
              const id = typeof raw === 'string' ? raw.trim() : ''
              if (id) void navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: id } })
            }}
          >
            <label className="block text-sm">
              Dispute ID
              <input name="disputeId" className={input} />
            </label>
            <button type="submit" className={button}>
              Show
            </button>
          </form>
        </section>
      </div>
    </AppShell>
  )
}
