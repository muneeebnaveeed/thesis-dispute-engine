import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import type { components } from '#/api/schema.gen'
import { TemplateEditor } from '#/components/comms/template-editor'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { tenantTemplatesQuery } from '#/queries/tenant-templates'
import { fieldErrorsOf, useServerMutation } from '#/queries/use-server-mutation'
import { deleteTenantTemplate, putTenantTemplate } from '#/server/functions/tenant-templates'

type NoticeKind = components['schemas']['NoticeKind']
type TemplateOverride = components['schemas']['TemplateOverride']

const TemplatesPage = () => {
  const { tenant } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false
  const { data: loadedSettings } = useSuspenseQuery(tenantTemplatesQuery())
  const [savedNote, setSavedNote] = useState<string | null>(null)
  const saveMutation = useServerMutation(
    (change: { kind: NoticeKind; override: TemplateOverride }) => putTenantTemplate({ data: change }),
    {
      invalidates: () => [tenantTemplatesQuery().queryKey],
      onSuccess: (setting) =>
        setSavedNote(`${setting.effective.label}: wording saved; analysts see it on their next email.`),
    },
  )
  const revertMutation = useServerMutation((kind: NoticeKind) => deleteTenantTemplate({ data: kind }), {
    invalidates: () => [tenantTemplatesQuery().queryKey],
    onSuccess: () => setSavedNote('Reverted to the standard wording.'),
  })
  const failure = saveMutation.failure ?? revertMutation.failure
  const fieldErrors = fieldErrorsOf(failure)

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
          {savedNote && (
            <output className="block rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
              {savedNote}
            </output>
          )}
          {failure && failure.kind !== 'validation' && (
            <FailureBanner
              failure={failure}
              onRetry={() => {
                saveMutation.clearFailure()
                revertMutation.clearFailure()
              }}
            />
          )}
          {loadedSettings.failure ? (
            <FailureBanner failure={loadedSettings.failure} />
          ) : (
            loadedSettings.value.map((setting) => (
              <TemplateEditor
                key={setting.base.kind}
                setting={setting}
                fields={fieldErrors}
                busy={saveMutation.isPending || revertMutation.isPending}
                onSave={(override) => {
                  setSavedNote(null)
                  saveMutation.mutate({ kind: setting.base.kind, override })
                }}
                onRevert={() => {
                  setSavedNote(null)
                  revertMutation.mutate(setting.base.kind)
                }}
              />
            ))
          )}
        </div>
      )}
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/templates')({
  loader: ({ context }) => context.queryClient.query({ ...tenantTemplatesQuery(), staleTime: 'static' }),
  component: TemplatesPage,
})
