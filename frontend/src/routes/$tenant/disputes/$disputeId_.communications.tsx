import { Link, createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useEffect, useMemo, useState } from 'react'

import { createBrowserApi } from '#/api/browser'
import { call } from '#/api/call'
import { classify, type Failure } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { AppShell } from '#/components/app-shell'
import { EmailComposer, type Draft } from '#/components/email-composer'
import { FailureBanner } from '#/components/failure-banner'
import { SentEmails } from '#/components/sent-emails'
import { TenantMismatch } from '#/components/tenant-mismatch'
import { getDispute } from '#/server/disputes'

type EmailTemplates = components['schemas']['EmailTemplates']
type Tab = 'create' | 'sent'

// The communications panel: compose from a template with a live preview, or read what has gone out.
export const Route = createFileRoute('/$tenant/disputes/$disputeId_/communications')({
  loader: ({ params }) => getDispute({ data: params.disputeId }),
  validateSearch: (s: Record<string, unknown>): { tab?: Tab } => (s.tab === 'sent' ? { tab: 'sent' } : {}),
  component: CommunicationsPage,
})

function CommunicationsPage() {
  const outcome = Route.useLoaderData()
  const { tenant, disputeId } = Route.useParams()
  const { tab = 'create' } = Route.useSearch()
  const { config, viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  const api = useMemo(
    () =>
      createBrowserApi(config.apiUrl, () =>
        window.location.assign(`/${tenant}?next=${encodeURIComponent(window.location.pathname)}`),
      ),
    [config.apiUrl, tenant],
  )
  const [catalogue, setCatalogue] = useState<EmailTemplates | null>(null)
  const [failure, setFailure] = useState<Failure | null>(null)
  const [busy, setBusy] = useState(false)
  const [sentNote, setSentNote] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Draft[]>([])

  useEffect(() => {
    let live = true
    call(() => api.GET('/disputes/{disputeId}/email-templates', { params: { path: { disputeId } } }))
      .then((res) => {
        if (!live) return undefined
        if (res.failure) setFailure(res.failure)
        else setCatalogue(res.data)
        return undefined
      })
      .catch(() => undefined)
    return () => {
      live = false
    }
  }, [api, disputeId])

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <AppShell title="Communications">
        {f && <FailureBanner failure={f} onRetry={() => void router.invalidate()} />}
      </AppShell>
    )
  }
  const d = outcome.value
  const fields: Record<string, string> = failure?.kind === 'validation' ? failure.fields : {}
  const showTab = (next: Tab) =>
    void router.navigate({
      to: '/$tenant/disputes/$disputeId/communications',
      params: { tenant, disputeId },
      search: next === 'sent' ? { tab: 'sent' } : {},
    })

  async function send(template: EmailTemplates['templates'][number]['kind'], inputs: Record<string, string>) {
    setBusy(true)
    setFailure(null)
    setSentNote(null)
    const res = await call(() =>
      api.POST('/disputes/{disputeId}/notices', {
        params: { path: { disputeId } },
        body: { template, fields: inputs, attachments: drafts.map((draft) => draft.id) },
      }),
    )
    setBusy(false)
    if (res.failure) {
      setFailure(res.failure)
      return
    }
    setSentNote('Email queued; it appears under Sent Emails as it goes out.')
    setDrafts([])
    await router.invalidate()
    showTab('sent')
  }

  async function upload(file: File) {
    setBusy(true)
    setFailure(null)
    const form = new FormData()
    form.append('file', file, file.name)
    const res = await call(() =>
      api.POST('/disputes/{disputeId}/attachments', {
        params: { path: { disputeId } },
        body: { file: file.name },
        bodySerializer: () => form,
      }),
    )
    setBusy(false)
    if (res.failure) {
      setFailure(res.failure)
      return
    }
    setDrafts((cur) => [...cur, { id: res.data.id, filename: res.data.filename, size: res.data.size }])
  }

  async function resend(noticeId: number) {
    setBusy(true)
    setFailure(null)
    setSentNote(null)
    const res = await call(() =>
      api.POST('/disputes/{disputeId}/notices/{noticeId}/resend', {
        params: { path: { disputeId, noticeId } },
      }),
    )
    setBusy(false)
    if (res.failure) {
      setFailure(res.failure)
      return
    }
    setSentNote('Email queued again; it appears below as it goes out.')
    await router.invalidate()
  }

  const tabClass = (t: Tab) =>
    `border-b-2 px-3 py-2 text-sm ${tab === t ? 'border-neutral-900 font-medium' : 'border-transparent text-neutral-500 hover:text-neutral-900'}`

  return (
    <AppShell title={`Communications for dispute ${d.id.slice(0, 8)}`}>
      <p className="mb-4 text-sm">
        <Link to="/$tenant/disputes/$disputeId" params={{ tenant, disputeId }} className="underline">
          Back to the dispute
        </Link>
        <span className="ml-3 text-neutral-500">
          {d.reason} / {d.regime} / {d.state}
        </span>
      </p>
      <div role="tablist" aria-label="Communications" className="mb-6 flex gap-2 border-b border-neutral-200">
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'create'}
          className={tabClass('create')}
          onClick={() => showTab('create')}
        >
          Create Email
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'sent'}
          className={tabClass('sent')}
          onClick={() => showTab('sent')}
        >
          Sent Emails
          <span className="ml-2 rounded-full bg-neutral-200 px-2 py-0.5 text-xs">{d.notices.length}</span>
        </button>
      </div>
      {sentNote && (
        <output className="mb-4 block rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
          {sentNote}
        </output>
      )}
      {failure && failure.kind !== 'validation' && (
        <div className="mb-4">
          <FailureBanner failure={failure} onRetry={() => setFailure(null)} />
        </div>
      )}
      {tab === 'create' ? (
        catalogue ? (
          <EmailComposer
            templates={catalogue.templates}
            facts={catalogue.facts}
            fields={fields}
            busy={busy}
            onSend={(t, i) => void send(t, i)}
            drafts={drafts}
            onUpload={(f) => void upload(f)}
            onRemoveDraft={(id) => setDrafts((cur) => cur.filter((draft) => draft.id !== id))}
          />
        ) : (
          <p className="text-sm text-neutral-500">Loading templates...</p>
        )
      ) : (
        <SentEmails
          notices={d.notices}
          api={api}
          disputeId={d.id}
          busy={busy}
          onResend={(id) => void resend(id)}
        />
      )}
    </AppShell>
  )
}
