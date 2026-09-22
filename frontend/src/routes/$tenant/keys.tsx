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
            <form className="flex items-start gap-2" onSubmit={submitting(issueForm)}>
              <issueForm.AppField name="label">
                {(field) => (
                  <field.TextField
                    label={<span className="sr-only">Label</span>}
                    inputClassName="mt-0 px-3 py-2"
                    placeholder="core banking production"
                  />
                )}
              </issueForm.AppField>
              <issueForm.AppForm>
                <issueForm.SubmitButton busy={issueMutation.isPending}>Issue</issueForm.SubmitButton>
              </issueForm.AppForm>
            </form>
            {issuedKey && (
              <output className="mt-4 block rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm">
                <p className="font-medium">
                  Key for {issuedKey.label}. Copy it now; it will not be shown again.
                </p>
                <code className="mt-2 block rounded bg-white p-2 font-mono break-all select-all">
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
  if (keys.length === 0) return <p className="text-sm text-muted-foreground">No keys yet.</p>
  return (
    <table className="w-full text-left text-sm">
      <thead className="text-muted-foreground">
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
        {keys.map((tenantKey) => (
          <tr key={tenantKey.id} className="border-t border-border">
            <td className="py-1 pr-4 font-mono">{tenantKey.prefix}</td>
            <td className="py-1 pr-4">{tenantKey.label}</td>
            <td className="py-1 pr-4 text-muted-foreground">{dateOnly(tenantKey.createdAt)}</td>
            <td className="py-1 pr-4 text-muted-foreground">{dateOnly(tenantKey.lastUsedAt)}</td>
            <td className="py-1 pr-4 text-muted-foreground">{dateOnly(tenantKey.expiresAt)}</td>
            <td className="py-1 pr-4">{tenantKey.status}</td>
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
