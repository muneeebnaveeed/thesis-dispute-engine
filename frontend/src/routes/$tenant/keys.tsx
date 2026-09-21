import { createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import type { Problem } from '#/api/problem'
import type { components } from '#/api/schema.gen'
import { AppShell } from '#/components/app-shell'
import { ProblemBanner } from '#/components/problem-banner'
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
  const [problem, setProblem] = useState<Problem | null>(null)
  const [issued, setIssued] = useState<{ label: string; secret: string } | null>(null)
  const [busy, setBusy] = useState<string | null>(null)

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false

  async function create(form: FormData) {
    const raw = form.get('label')
    const label = typeof raw === 'string' ? raw.trim() : ''
    if (!label) return
    setBusy('create')
    setProblem(null)
    setIssued(null)
    const { data, error } = await api.POST('/tenant-keys', { body: { label } })
    setBusy(null)
    if (error) setProblem(error)
    else if (data) {
      setIssued({ label: data.label, secret: data.secret })
      await router.invalidate()
    }
  }

  async function revoke(id: string) {
    setBusy(id)
    setProblem(null)
    const { error } = await api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId: id } } })
    setBusy(null)
    if (error) setProblem(error)
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
                className={input}
                placeholder="core banking production"
                aria-label="Label"
              />
              <button type="submit" className={button} disabled={busy !== null}>
                Issue
              </button>
            </form>
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
            {problem && (
              <div className="mt-4">
                <ProblemBanner problem={problem} />
              </div>
            )}
          </section>
          <section>
            <h2 className="mb-2 text-lg font-medium">Keys</h2>
            {outcome.problem ? (
              <ProblemBanner problem={outcome.problem} />
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
