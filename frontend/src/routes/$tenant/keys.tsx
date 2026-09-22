import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import type { components } from '#/api/schema.gen'
import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { Panel } from '#/components/ui/panel'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Button } from '#/components/ui/button'
import { submitTo, submitting, useAppForm } from '#/forms/app-form'
import { parsed, schemaValidator } from '#/forms/schema'
import { tenantKeysQuery } from '#/queries/tenant-keys'
import { useServerMutation } from '#/queries/use-server-mutation'
import { issueTenantKey, revokeTenantKey } from '#/server/functions/tenant-keys'

type TenantKey = components['schemas']['TenantKey']

const KeysPage = () => {
  const { tenant } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const { data: loadedKeys } = useSuspenseQuery(tenantKeysQuery())
  const [issuedKey, setIssuedKey] = useState<{ label: string; secret: string } | null>(null)
  const issueMutation = useServerMutation((request: { label: string }) => issueTenantKey({ data: request }), {
    invalidates: () => [tenantKeysQuery().queryKey],
    onSuccess: (issued) => setIssuedKey({ label: issued.label, secret: issued.secret }),
  })
  const issueForm = useAppForm({
    defaultValues: { label: '' },
    validators: { onSubmit: schemaValidator(CreateTenantKeyRequest) },
    onSubmit: async ({ value, formApi }) => {
      setIssuedKey(null)
      await submitTo(formApi, () => issueMutation.mutateAsync(parsed(CreateTenantKeyRequest, value)))
    },
  })
  const revokeMutation = useServerMutation((keyId: string) => revokeTenantKey({ data: keyId }), {
    invalidates: () => [tenantKeysQuery().queryKey],
  })
  const failure = issueMutation.failure ?? revokeMutation.failure

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false

  return (
    <AppShell title="Tenant keys">
      {!isAdmin ? (
        <p className="text-sm text-muted-foreground">
          Only administrators of your organisation can manage keys.
        </p>
      ) : (
        <div className="space-y-6">
          <Panel title="Issue a key">
            <p className="mb-3 text-sm text-muted-foreground">
              One key per system that calls the API. The key is shown once; store it in that system's
              configuration.
            </p>
            <form className="form-rows" onSubmit={submitting(issueForm)}>
              <issueForm.AppField name="label">
                {(field) => <field.TextField label="Label" placeholder="core banking production" />}
              </issueForm.AppField>
              <div className="mt-2 flex justify-end border-t border-border pt-2">
                <issueForm.AppForm>
                  <issueForm.SubmitButton busy={issueMutation.isPending}>Issue</issueForm.SubmitButton>
                </issueForm.AppForm>
              </div>
            </form>
            {issuedKey && (
              <output className="mt-2 block border border-emerald-300 bg-emerald-50 p-2 text-[11px]">
                <p className="font-medium">
                  Key for {issuedKey.label}. Copy it now; it will not be shown again.
                </p>
                <code className="mt-1 block border border-emerald-200 bg-white p-1 font-mono break-all select-all">
                  {issuedKey.secret}
                </code>
              </output>
            )}
            {failure && failure.kind !== 'validation' && (
              <div className="mt-4">
                <FailureBanner
                  failure={failure}
                  onRetry={() => {
                    issueMutation.clearFailure()
                    revokeMutation.clearFailure()
                  }}
                />
              </div>
            )}
          </Panel>
          <Panel title="Keys" bodyClassName="p-0">
            {loadedKeys.failure ? (
              <FailureBanner failure={loadedKeys.failure} />
            ) : (
              <KeyTable
                keys={loadedKeys.value}
                busy={revokeMutation.isPending}
                onRevoke={(keyId) => revokeMutation.mutate(keyId)}
              />
            )}
          </Panel>
        </div>
      )}
    </AppShell>
  )
}

const dateOnly = (timestamp?: string) => (timestamp ? timestamp.slice(0, 10) : '-')

const KeyTable = ({
  keys,
  busy,
  onRevoke,
}: {
  keys: TenantKey[]
  busy: boolean
  onRevoke: (keyId: string) => void
}) => {
  if (keys.length === 0) return <p className="p-2 text-[11px] text-muted-foreground">No keys yet.</p>
  return (
    <table className="w-full border-collapse text-left text-[11px] [&_td]:border-t [&_td]:border-[color:var(--rule)] [&_td]:border-r [&_td]:px-1.5 [&_td]:py-[3px] [&_th]:border-r [&_th]:border-b [&_th]:border-border [&_th]:px-1.5 [&_th]:py-[3px]">
      <thead className="bg-[image:var(--toolbar)] text-foreground">
        <tr>
          <th className="font-bold">Prefix</th>
          <th className="font-bold">Label</th>
          <th className="font-bold">Created</th>
          <th className="font-bold">Last used</th>
          <th className="font-bold">Expires</th>
          <th className="font-bold">Status</th>
          <th className="font-bold">
            <span className="sr-only">Actions</span>
          </th>
        </tr>
      </thead>
      <tbody>
        {keys.map((tenantKey) => (
          <tr key={tenantKey.id} className="odd:bg-muted hover:bg-accent">
            <td className="font-mono">{tenantKey.prefix}</td>
            <td>{tenantKey.label}</td>
            <td className="text-muted-foreground">{dateOnly(tenantKey.createdAt)}</td>
            <td className="text-muted-foreground">{dateOnly(tenantKey.lastUsedAt)}</td>
            <td className="text-muted-foreground">{dateOnly(tenantKey.expiresAt)}</td>
            <td>{tenantKey.status}</td>
            <td className="py-1 text-right">
              {tenantKey.status === 'live' && (
                <Button variant="secondary" size="xs" disabled={busy} onClick={() => onRevoke(tenantKey.id)}>
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

export const Route = createFileRoute('/$tenant/keys')({
  loader: ({ context }) => context.queryClient.query({ ...tenantKeysQuery(), staleTime: 'static' }),
  component: KeysPage,
})
