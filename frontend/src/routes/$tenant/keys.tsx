import { createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import { call } from '#/api/call'
import { classify, type Failure } from '#/api/failure'
import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import type { components } from '#/api/schema.gen'
import { AppShell } from '#/components/app-shell'
import { FailureBanner, FieldError } from '#/components/failure-banner'
import { serverFields, validateForm } from '#/forms/validate-form'
import { TenantMismatch } from '#/components/tenant-mismatch'
import { listTenantKeys } from '#/server/tenant-keys'

type TenantKey = components['schemas']['TenantKey']

// Tenant admins manage the keys their own systems use. The first paint lists them server-side; creating and
// revoking go straight from the browser to the API with the analyst's token, and the secret is shown exactly once.
export const Route = createFileRoute('/$tenant/keys')({
  loader: () => listTenantKeys(),
  component: KeysPage,
})

const input =
  'w-full rounded-md border border-neutral-300 px-3 py-2 text-sm focus:border-neutral-500 focus:outline-none'
const button =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50'

function KeysPage() {
  const outcome = Route.useLoaderData()
  const { tenant } = Route.useParams()
  const { config, viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  const api = useMemo(
    () =>
      createBrowserApi(config.apiUrl, () =>
        window.location.assign(`/${tenant}?next=${encodeURIComponent(window.location.pathname)}`),
      ),
    [config.apiUrl, tenant],
  )
  const [failure, setFailure] = useState<Failure | null>(null)
  const [fields, setFields] = useState<Record<string, string>>({})
  const [issued, setIssued] = useState<{ label: string; secret: string } | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false

  async function create(form: FormData) {
    setFailure(null)
    setIssued(null)
    const checked = validateForm(CreateTenantKeyRequest, form)
    setFields(checked.fields)
    if (!checked.value) return
    setBusy('create')
    const res = await call(() => api.POST('/tenant-keys', { body: checked.value }))
    setBusy(null)
    if (res.failure) {
      setFailure(res.failure)
      if (res.failure.kind === 'validation')
        setFields({ ...res.failure.fields, ...serverFields(res.failure.problem) })
    } else {
      setIssued({ label: res.data.label, secret: res.data.secret })
      await router.invalidate()
    }
  }

  async function revoke(id: string) {
    setBusy(id)
    setFailure(null)
    const res = await call(() => api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId: id } } }), {
      idempotent: true,
    })
    setBusy(null)
    if (res.failure) setFailure(res.failure)
    else await router.invalidate()
  }

  return (
    <AppShell title="Tenant keys">
      {!isAdmin ? (
        <p className="text-sm text-neutral-600">Only administrators of your organisation can manage keys.</p>
      ) : (
        <div className="space-y-8">
          <section>
            <h2 className="mb-2 text-lg font-medium">Issue a key</h2>
            <p className="mb-3 text-sm text-neutral-600">
              One key per system that calls the API. The key is shown once; store it in that system's
              configuration.
            </p>
            <form
              className="flex gap-2"
              onSubmit={(e) => {
                e.preventDefault()
                void create(new FormData(e.currentTarget))
              }}
            >
              <input
                name="label"
                className={`${input} ${fields.label ? 'border-red-400' : ''}`}
                placeholder="core banking production"
                aria-label="Label"
                aria-invalid={fields.label ? true : undefined}
                aria-describedby={fields.label ? 'label-error' : undefined}
              />
              <button type="submit" className={button} disabled={busy !== null}>
                Issue
              </button>
            </form>
            <FieldError id="label-error" message={fields.label} />
            {issued && (
              <output className="mt-4 block rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm">
                <p className="font-medium">
                  Key for {issued.label}. Copy it now; it will not be shown again.
                </p>
                <code className="mt-2 block rounded bg-white p-2 font-mono break-all select-all">
                  {issued.secret}
                </code>
              </output>
            )}
            {failure && failure.kind !== 'validation' && (
              <div className="mt-4">
                <FailureBanner failure={failure} onRetry={() => setFailure(null)} />
              </div>
            )}
          </section>
          <section>
            <h2 className="mb-2 text-lg font-medium">Keys</h2>
            {outcome.problem ? (
              <FailureBanner
                failure={
                  classify({ error: outcome.problem }) ?? {
                    kind: 'unexpected',
                    status: 0,
                    message: 'no data',
                  }
                }
              />
            ) : (
              <KeyTable keys={outcome.value ?? []} busy={busy} onRevoke={(id) => void revoke(id)} />
            )}
          </section>
        </div>
      )}
    </AppShell>
  )
}

const day = (s?: string) => (s ? s.slice(0, 10) : '-')

function KeyTable({
  keys,
  busy,
  onRevoke,
}: {
  keys: TenantKey[]
  busy: string | null
  onRevoke: (id: string) => void
}) {
  if (keys.length === 0) return <p className="text-sm text-neutral-600">No keys yet.</p>
  return (
    <table className="w-full text-left text-sm">
      <thead className="text-neutral-500">
        <tr>
          <th className="py-1 pr-4 font-normal">Prefix</th>
          <th className="py-1 pr-4 font-normal">Label</th>
          <th className="py-1 pr-4 font-normal">Created</th>
          <th className="py-1 pr-4 font-normal">Last used</th>
          <th className="py-1 pr-4 font-normal">Expires</th>
          <th className="py-1 pr-4 font-normal">Status</th>
          <th className="py-1 font-normal">
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {keys.map((k) => (
          <tr key={k.id} className="border-t border-neutral-200">
            <td className="py-1 pr-4 font-mono">{k.prefix}</td>
            <td className="py-1 pr-4">{k.label}</td>
            <td className="py-1 pr-4 text-neutral-500">{day(k.createdAt)}</td>
            <td className="py-1 pr-4 text-neutral-500">{day(k.lastUsedAt)}</td>
            <td className="py-1 pr-4 text-neutral-500">{day(k.expiresAt)}</td>
            <td className="py-1 pr-4">{k.status}</td>
            <td className="py-1 text-right">
              {k.status === 'live' && (
                <button
                  type="button"
                  disabled={busy !== null}
                  onClick={() => onRevoke(k.id)}
                  className="rounded-md border border-neutral-300 bg-white px-2 py-1 text-xs hover:bg-neutral-100 disabled:opacity-50"
                >
                  Revoke
                </button>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
