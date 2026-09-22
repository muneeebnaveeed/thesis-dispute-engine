import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { classify } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { EmailComposer, type Draft } from '#/components/comms/email-composer'
import { SentEmails } from '#/components/comms/sent-emails'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Badge } from '#/components/ui/badge'
import { cn } from '#/lib/cn'
import { disputeQuery, emailTemplatesQuery } from '#/queries'
import { fieldsOf, useServerMutation } from '#/queries/mutation'
import { composeEmail, resendNotice, uploadAttachment } from '#/server/functions/mutations'

type Tab = 'create' | 'sent'
type Kind = components['schemas']['NoticeKind']

const CommunicationsPage = () => {
  const { tenant, disputeId } = Route.useParams()
  const { tab = 'create' } = Route.useSearch()
  const { viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  const queryClient = useQueryClient()
  const { data: outcome } = useSuspenseQuery(disputeQuery(disputeId))
  const { data: catalogue } = useSuspenseQuery(emailTemplatesQuery(disputeId))
  const [sentNote, setSentNote] = useState<string | null>(null)
  const [drafts, setDrafts] = useState<Draft[]>([])

  const showTab = (next: Tab) =>
    router.navigate({
      to: '/$tenant/disputes/$disputeId/communications',
      params: { tenant, disputeId },
      search: next === 'sent' ? { tab: 'sent' } : {},
    })

  const send = useServerMutation(
    (vars: { template: Kind; fields: Record<string, string> }) =>
      composeEmail({
        data: {
          disputeId,
          body: { template: vars.template, fields: vars.fields, attachments: drafts.map((d) => d.id) },
        },
      }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: async () => {
        setSentNote('Email queued; it appears under Sent Emails as it goes out.')
        setDrafts([])
        await showTab('sent')
      },
    },
  )
  const resend = useServerMutation((noticeId: number) => resendNotice({ data: { disputeId, noticeId } }), {
    invalidates: () => [disputeQuery(null).queryKey],
    onSuccess: () => setSentNote('Email queued again; it appears below as it goes out.'),
  })
  const upload = useServerMutation(
    (file: File) => {
      const form = new FormData()
      form.append('disputeId', disputeId)
      form.append('file', file, file.name)
      return uploadAttachment({ data: form })
    },
    { onSuccess: (a) => setDrafts((cur) => [...cur, { id: a.id, filename: a.filename, size: a.size }]) },
  )
  const failure = send.failure ?? upload.failure ?? resend.failure
  const clearFailures = () => {
    send.clearFailure()
    upload.clearFailure()
    resend.clearFailure()
  }
  const busy = send.isPending || upload.isPending || resend.isPending

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (outcome.problem || !outcome.value) {
    const f = outcome.problem ? classify({ error: outcome.problem }) : null
    return (
      <AppShell title="Communications">
        {f && (
          <FailureBanner
            failure={f}
            onRetry={() => void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })}
          />
        )}
      </AppShell>
    )
  }
  const d = outcome.value
  const fields = fieldsOf(failure)
  const tabClass = (t: Tab) =>
    cn(
      'border-b-2 px-3 py-2 text-sm',
      tab === t
        ? 'border-neutral-900 font-medium'
        : 'border-transparent text-neutral-500 hover:text-neutral-900',
    )

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
          onClick={() => void showTab('create')}
        >
          Create Email
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={tab === 'sent'}
          className={tabClass('sent')}
          onClick={() => void showTab('sent')}
        >
          Sent Emails
          <Badge tone="neutral" className="ml-2 rounded-full px-2">
            {d.notices.length}
          </Badge>
        </button>
      </div>
      {sentNote && (
        <output className="mb-4 block rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
          {sentNote}
        </output>
      )}
      {failure && failure.kind !== 'validation' && (
        <div className="mb-4">
          <FailureBanner failure={failure} onRetry={clearFailures} />
        </div>
      )}
      {tab === 'create' ? (
        catalogue.value ? (
          <EmailComposer
            templates={catalogue.value.templates}
            facts={catalogue.value.facts}
            fields={fields}
            busy={busy}
            onSend={(template, inputs) => send.mutate({ template, fields: inputs })}
            drafts={drafts}
            onUpload={(f) => upload.mutate(f)}
            onRemoveDraft={(id) => setDrafts((cur) => cur.filter((draft) => draft.id !== id))}
          />
        ) : (
          <FailureBanner
            failure={
              classify({ error: catalogue.problem }) ?? {
                kind: 'unexpected',
                status: 0,
                message: 'no templates',
              }
            }
          />
        )
      ) : (
        <SentEmails notices={d.notices} disputeId={d.id} busy={busy} onResend={(id) => resend.mutate(id)} />
      )}
    </AppShell>
  )
}

// The communications panel: compose from a template with a live preview, or read what has gone out.
export const Route = createFileRoute('/$tenant/disputes/$disputeId_/communications')({
  validateSearch: (s: Record<string, unknown>): { tab?: Tab } => (s.tab === 'sent' ? { tab: 'sent' } : {}),
  loader: ({ params, context }) =>
    Promise.all([
      context.queryClient.query({ ...disputeQuery(params.disputeId), staleTime: 'static' }),
      context.queryClient.query({ ...emailTemplatesQuery(params.disputeId), staleTime: 'static' }),
    ]),
  component: CommunicationsPage,
})
