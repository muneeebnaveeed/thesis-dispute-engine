import { createFileRoute, useRouteContext } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import { call } from '#/api/call'
import type { Failure } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { AppShell } from '#/components/app-shell'
import { FailureBanner } from '#/components/failure-banner'
import { TemplateEditor } from '#/components/template-editor'
import { TenantMismatch } from '#/components/tenant-mismatch'

type TemplateSetting = components['schemas']['TemplateSetting']
type TemplateOverride = components['schemas']['TemplateOverride']

// Tenant admins reword the analyst email templates: the words are theirs, the form stays the engine's.
export const Route = createFileRoute('/$tenant/templates')({ component: TemplatesPage })

function TemplatesPage() {
  const { tenant } = Route.useParams()
  const { config, viewer } = useRouteContext({ from: '__root__' })
  const api = useMemo(
    () =>
      createBrowserApi(config.apiUrl, () =>
        window.location.assign(`/${tenant}?next=${encodeURIComponent(window.location.pathname)}`),
      ),
    [config.apiUrl, tenant],
  )
  const [settings, setSettings] = useState<TemplateSetting[] | null>(null)
  const [failure, setFailure] = useState<Failure | null>(null)
  const [busy, setBusy] = useState(false)
  const [note, setNote] = useState<string | null>(null)
  const isAdmin = viewer?.roles.includes('tenant-admin') ?? false

  useEffect(() => {
    if (!isAdmin) return undefined
    let live = true
    call(() => api.GET('/tenant-templates'))
      .then((res) => {
        if (!live) return undefined
        if (res.failure) setFailure(res.failure)
        else setSettings(res.data)
        return undefined
      })
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [api, isAdmin])

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  async function save(kind: TemplateSetting['base']['kind'], override: TemplateOverride) {
    setBusy(true)
    setFailure(null)
    setNote(null)
    const res = await call(() =>
      api.PUT('/tenant-templates/{kind}', { params: { path: { kind } }, body: override }),
    )
    setBusy(false)
    if (res.failure) {
      setFailure(res.failure)
      return
    }
    setSettings((cur) => cur?.map((s) => (s.base.kind === kind ? res.data : s)) ?? null)
    setNote(`${res.data.effective.label}: wording saved; analysts see it on their next email.`)
  }

  async function revert(kind: TemplateSetting['base']['kind']) {
    setBusy(true)
    setFailure(null)
    setNote(null)
    const res = await call(() => api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } }), {
      idempotent: true,
    })
    setBusy(false)
    if (res.failure) {
      setFailure(res.failure)
      return
    }
    setSettings(
      (cur) => cur?.map((s) => (s.base.kind === kind ? { base: s.base, effective: s.base } : s)) ?? null,
    )
    setNote('Reverted to the standard wording.')
  }

  const fields: Record<string, string> = failure?.kind === 'validation' ? failure.fields : {}

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
            <FailureBanner failure={failure} onRetry={() => setFailure(null)} />
          )}
          {settings ? (
            settings.map((s) => (
              <TemplateEditor
                key={s.base.kind}
                setting={s}
                fields={fields}
                busy={busy}
                onSave={(o) => void save(s.base.kind, o)}
                onRevert={() => void revert(s.base.kind)}
              />
            ))
          ) : (
            <p className="text-sm text-neutral-500">Loading...</p>
          )}
        </div>
      )}
    </AppShell>
  )
}
