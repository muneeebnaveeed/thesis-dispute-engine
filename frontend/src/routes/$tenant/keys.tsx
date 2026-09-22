import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import { classify } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner, FieldError } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'
import { serverFields, validateForm } from '#/forms/validate-form'
import { cn } from '#/lib/cn'
import { tenantKeysQuery } from '#/queries'
import { fieldsOf, useServerMutation } from '#/queries/mutation'
import { issueTenantKey, revokeTenantKey } from '#/server/functions/mutations'

type TenantKey = components['schemas']['TenantKey']

const KeysPage = () => {
  const { tenant } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: outcome } = useSuspenseQuery(tenantKeysQuery())
  const [localFields, setLocalFields] = useState<Record<string, string>>({})
  const [issued, setIssued] = useState<{ label: string; secret: string } | null>(null)
  const issue = useServerMutation((body: { label: string }) => issueTenantKey({ data: body }), {
    invalidates: () => [tenantKeysQuery().queryKey],
    onSuccess: (key) => setIssued({ label: key.label, secret: key.secret }),
  })
  const revoke = useServerMutation((id: string) => revokeTenantKey({ data: id }), {
    invalidates: () => [tenantKeysQuery().queryKey],
  })
  const fields = {
    ...localFields,
    ...fieldsOf(issue.failure),
    ...(issue.failure?.kind === 'validation' ? serverFields(issue.failure.problem) : {}),
  }
  const failure = issue.failure ?? revoke.failure

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false

  const submit = (form: FormData) => {
    setIssued(null)
    const checked = validateForm(CreateTenantKeyRequest, form)
    setLocalFields(checked.fields)
    if (checked.value) issue.mutate(checked.value)
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
                submit(new FormData(e.currentTarget))
              }}
            >
              <input
                name="label"
                className={cn(inputVariants({ invalid: Boolean(fields.label) }), 'mt-0 px-3 py-2')}
                placeholder="core banking production"
                aria-label="Label"
                {...invalidProps('label', fields.label)}
              />
              <Button type="submit" disabled={issue.isPending}>
                Issue
              </Button>
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
                <FailureBanner
                  failure={failure}
                  onRetry={() => {
                    issue.clearFailure()
                    revoke.clearFailure()
                  }}
                />
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
              <KeyTable
                keys={outcome.value ?? []}
                busy={revoke.isPending}
                onRevoke={(id) => revoke.mutate(id)}
              />
            )}
          </section>
        </div>
      )}
    </AppShell>
  )
}

const day = (s?: string) => (s ? s.slice(0, 10) : '-')

const KeyTable = ({
  keys,
  busy,
  onRevoke,
}: {
  keys: TenantKey[]
  busy: boolean
  onRevoke: (id: string) => void
}) => {
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
                <Button variant="secondary" size="xs" disabled={busy} onClick={() => onRevoke(k.id)}>
                  Revoke
                </Button>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

// Tenant admins manage the keys their own systems use; the secret is shown exactly once.
export const Route = createFileRoute('/$tenant/keys')({
  loader: ({ context }) => context.queryClient.ensureQueryData(tenantKeysQuery()),
  component: KeysPage,
})
