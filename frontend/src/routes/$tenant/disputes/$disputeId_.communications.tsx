import { useQueryClient, useSuspenseQuery } from '@tanstack/react-query'
import { Link, createFileRoute, useRouteContext, useRouter } from '@tanstack/react-router'
import { useState } from 'react'

import { classify } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { EmailComposer, type AttachmentDraft } from '#/components/comms/email-composer'
import { SentEmails } from '#/components/comms/sent-emails'
import { AppShell } from '#/components/layout/app-shell'
import { FailureBanner } from '#/components/layout/failure-banner'
import { TenantMismatch } from '#/components/layout/tenant-mismatch'
import { Badge } from '#/components/ui/badge'
import { cn } from '#/lib/cn'
import { disputeQuery } from '#/queries/disputes'
import { emailTemplatesQuery } from '#/queries/notices'
import { fieldErrorsOf, useServerMutation } from '#/queries/use-server-mutation'
import { composeEmail, resendNotice, uploadAttachment } from '#/server/functions/notices'

type CommunicationsTab = 'create' | 'sent'
type NoticeKind = components['schemas']['NoticeKind']

const CommunicationsPage = () => {
  const { tenant, disputeId } = Route.useParams()
  const { tab: activeTab = 'create' } = Route.useSearch()
  const { viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  const queryClient = useQueryClient()
  const { data: disputeOutcome } = useSuspenseQuery(disputeQuery(disputeId))
  const { data: catalogueOutcome } = useSuspenseQuery(emailTemplatesQuery(disputeId))
  const [queuedNote, setQueuedNote] = useState<string | null>(null)
  const [attachmentDrafts, setAttachmentDrafts] = useState<AttachmentDraft[]>([])

  const showTab = (tab: CommunicationsTab) =>
    router.navigate({
      to: '/$tenant/disputes/$disputeId/communications',
      params: { tenant, disputeId },
      search: tab === 'sent' ? { tab: 'sent' } : {},
    })

  const sendMutation = useServerMutation(
    ({ template, fields }: { template: NoticeKind; fields: Record<string, string> }) =>
      composeEmail({
        data: {
          disputeId,
          body: { template, fields, attachments: attachmentDrafts.map((draft) => draft.id) },
        },
      }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: async () => {
        setQueuedNote('Email queued; it appears under Sent Emails as it goes out.')
        setAttachmentDrafts([])
        await showTab('sent')
      },
    },
  )
  const resendMutation = useServerMutation(
    (noticeId: number) => resendNotice({ data: { disputeId, noticeId } }),
    {
      invalidates: () => [disputeQuery(null).queryKey],
      onSuccess: () => setQueuedNote('Email queued again; it appears below as it goes out.'),
    },
  )
  const uploadMutation = useServerMutation(
    (file: File) => {
      const multipart = new FormData()
      multipart.append('disputeId', disputeId)
      multipart.append('file', file, file.name)
      return uploadAttachment({ data: multipart })
    },
    {
      onSuccess: (uploaded) =>
        setAttachmentDrafts((current) => [
          ...current,
          { id: uploaded.id, filename: uploaded.filename, size: uploaded.size },
        ]),
    },
  )
  const failure = sendMutation.failure ?? uploadMutation.failure ?? resendMutation.failure
  const clearFailures = () => {
    sendMutation.clearFailure()
    uploadMutation.clearFailure()
    resendMutation.clearFailure()
  }
  const busy = sendMutation.isPending || uploadMutation.isPending || resendMutation.isPending

  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />
  if (disputeOutcome.problem || !disputeOutcome.value) {
    const loadFailure = disputeOutcome.problem ? classify({ error: disputeOutcome.problem }) : null
    return (
      <AppShell title="Communications">
        {loadFailure && (
          <FailureBanner
            failure={loadFailure}
            onRetry={() => void queryClient.invalidateQueries({ queryKey: disputeQuery(disputeId).queryKey })}
          />
        )}
      </AppShell>
    )
  }
  const dispute = disputeOutcome.value
  const fieldErrors = fieldErrorsOf(failure)
  const tabClass = (tab: CommunicationsTab) =>
    cn(
      'border-b-2 px-3 py-2 text-sm',
      activeTab === tab
        ? 'border-neutral-900 font-medium'
        : 'border-transparent text-neutral-500 hover:text-neutral-900',
    )

  return (
    <AppShell title={`Communications for dispute ${dispute.id.slice(0, 8)}`}>
      <p className="mb-4 text-sm">
        <Link to="/$tenant/disputes/$disputeId" params={{ tenant, disputeId }} className="underline">
          Back to the dispute
        </Link>
        <span className="ml-3 text-neutral-500">
          {dispute.reason} / {dispute.regime} / {dispute.state}
        </span>
      </p>
      <div role="tablist" aria-label="Communications" className="mb-6 flex gap-2 border-b border-neutral-200">
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'create'}
          className={tabClass('create')}
          onClick={() => void showTab('create')}
        >
          Create Email
        </button>
        <button
          type="button"
          role="tab"
          aria-selected={activeTab === 'sent'}
          className={tabClass('sent')}
          onClick={() => void showTab('sent')}
        >
          Sent Emails
          <Badge tone="neutral" className="ml-2 rounded-full px-2">
            {dispute.notices.length}
          </Badge>
        </button>
      </div>
      {queuedNote && (
        <output className="mb-4 block rounded-md border border-emerald-200 bg-emerald-50 p-3 text-sm text-emerald-900">
          {queuedNote}
        </output>
      )}
      {failure && failure.kind !== 'validation' && (
        <div className="mb-4">
          <FailureBanner failure={failure} onRetry={clearFailures} />
        </div>
      )}
      {activeTab === 'create' ? (
        catalogueOutcome.value ? (
          <EmailComposer
            templates={catalogueOutcome.value.templates}
            facts={catalogueOutcome.value.facts}
            fields={fieldErrors}
            busy={busy}
            onSend={(template, fields) => sendMutation.mutate({ template, fields })}
            attachmentDrafts={attachmentDrafts}
            onUpload={(file) => uploadMutation.mutate(file)}
            onRemoveAttachmentDraft={(draftId) =>
              setAttachmentDrafts((current) => current.filter((draft) => draft.id !== draftId))
            }
          />
        ) : (
          <FailureBanner
            failure={
              classify({ error: catalogueOutcome.problem }) ?? {
                kind: 'unexpected',
                status: 0,
                message: 'no templates',
              }
            }
          />
        )
      ) : (
        <SentEmails
          notices={dispute.notices}
          disputeId={dispute.id}
          busy={busy}
          onResend={(noticeId) => resendMutation.mutate(noticeId)}
        />
      )}
    </AppShell>
  )
}

export const Route = createFileRoute('/$tenant/disputes/$disputeId_/communications')({
  validateSearch: (rawSearch: Record<string, unknown>): { tab?: CommunicationsTab } =>
    rawSearch.tab === 'sent' ? { tab: 'sent' } : {},
  loader: ({ params, context }) =>
    Promise.all([
      context.queryClient.query({ ...disputeQuery(params.disputeId), staleTime: 'static' }),
      context.queryClient.query({ ...emailTemplatesQuery(params.disputeId), staleTime: 'static' }),
    ]),
  component: CommunicationsPage,
})
