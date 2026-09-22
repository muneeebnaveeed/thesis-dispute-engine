import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import { classify } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { TemplateEditor } from '#/components/comms/template-editor'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { tenantTemplatesQuery } from '#/queries'
import { fieldsOf, useServerMutation } from '#/queries/mutation'
import { deleteTenantTemplate, putTenantTemplate } from '#/server/functions/mutations'

type Kind = components['schemas']['NoticeKind']
type TemplateOverride = components['schemas']['TemplateOverride']

// Tenant admins reword the analyst email templates: the words are theirs, the form stays the engine's.
const TemplatesPage = () => {
  const { tenant } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false
  const { data: outcome } = useSuspenseQuery(tenantTemplatesQuery())
  const [note, setNote] = useState<string | null>(null)
  const save = useServerMutation(
    (vars: { kind: Kind; override: TemplateOverride }) => putTenantTemplate({ data: vars }),
    {
      invalidates: () => [tenantTemplatesQuery().queryKey],
      onSuccess: (setting) =>
        setNote(`${setting.effective.label}: wording saved; analysts see it on their next email.`),
    },
  )
  const revert = useServerMutation((kind: Kind) => deleteTenantTemplate({ data: kind }), {
    invalidates: () => [tenantTemplatesQuery().queryKey],
    onSuccess: () => setNote('Reverted to the standard wording.'),
  })
  const failure = save.failure ?? revert.failure
  const fields = fieldsOf(failure)

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  return (
    <AppShell title="Email templates">
      {!isAdmin ? (
        <p className="text-sm text-neutral-600">
          Only tenant administrators can change the wording of emails.
        </p>
      ) : (
        <div className="space-y-8">
          <p className="max-w-2xl text-sm text-neutral-600">
            These are the emails analysts compose from the dispute page. You can change the words; the fields
            an analyst fills in stay the same, so use their placeholders where the answer belongs.
          </p>
          {note && (
            <output className="block rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
              {note}
            </output>
          )}
          {failure && failure.kind !== 'validation' && (
            <FailureBanner
              failure={failure}
              onRetry={() => {
                save.clearFailure()
                revert.clearFailure()
              }}
            />
          )}
          {outcome.value ? (
            outcome.value.map((s) => (
              <TemplateEditor
                key={s.base.kind}
                setting={s}
                fields={fields}
                busy={save.isPending || revert.isPending}
                onSave={(o) => {
                  setNote(null)
                  save.mutate({ kind: s.base.kind, override: o })
                }}
                onRevert={() => {
                  setNote(null)
                  revert.mutate(s.base.kind)
                }}
              />
            ))
          ) : (
            <FailureBanner
              failure={
                classify({ error: outcome.problem }) ?? {
                  kind: 'unexpected',
                  status: 0,
                  message: 'no templates',
                }
              }
            />
          )}
        </div>
      )}
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/templates')({
  loader: ({ context }) => context.queryClient.ensureQueryData(tenantTemplatesQuery()),
  component: TemplatesPage,
})
