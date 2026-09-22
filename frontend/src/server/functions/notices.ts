import { Type } from '@sinclair/typebox'

import type { components } from '#/api/schema.gen'
import { ComposeEmailRequest } from '#/api/schemas.gen'
import { disputeView, type Dispute, type Outcome } from '#/api/views'
import { analystGet, analystPost, parse, asAnalyst, uuid } from '#/server/runtime/fn'

const NoticeRef = Type.Object({ disputeId: uuid, noticeId: Type.Integer({ minimum: 1 }) })
const AttachmentRef = Type.Object({ disputeId: uuid, attachmentId: uuid })

export type AttachmentBytes = { contentType: string; base64: string }

export const getNotice = analystGet
  .validator(parse(NoticeRef))
  .handler(
    ({ data: noticeRef, context }): Promise<Outcome<components['schemas']['NoticeDocument']>> =>
      asAnalyst(context.api, (api) =>
        api.GET('/disputes/{disputeId}/notices/{noticeId}', { params: { path: noticeRef } }),
      ),
  )

export const listEmailTemplates = analystGet
  .validator(parse(uuid))
  .handler(
    ({ data: disputeId, context }): Promise<Outcome<components['schemas']['EmailTemplates']>> =>
      asAnalyst(context.api, (api) =>
        api.GET('/disputes/{disputeId}/email-templates', { params: { path: { disputeId } } }),
      ),
  )

export const composeEmail = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ComposeEmailRequest })))
  .handler(
    ({ data: { disputeId, body }, context }): Promise<Outcome<Dispute>> =>
      asAnalyst(
        context.api,
        (api) => api.POST('/disputes/{disputeId}/notices', { params: { path: { disputeId } }, body }),
        disputeView,
      ),
  )

export const resendNotice = analystPost
  .validator(parse(NoticeRef))
  .handler(
    ({ data: noticeRef, context }): Promise<Outcome<Dispute>> =>
      asAnalyst(
        context.api,
        (api) => api.POST('/disputes/{disputeId}/notices/{noticeId}/resend', { params: { path: noticeRef } }),
        disputeView,
      ),
  )

export const getAttachment = analystGet
  .validator(parse(AttachmentRef))
  .handler(async ({ data: attachmentRef, context }): Promise<Outcome<AttachmentBytes>> => {
    if (!context.api) return asAnalyst(null, () => Promise.resolve({}))
    const response = await context.api.GET('/disputes/{disputeId}/attachments/{attachmentId}', {
      params: { path: attachmentRef },
      parseAs: 'blob',
    })
    if (response.error || !response.data) return { value: null, problem: response.error ?? null }
    return {
      value: {
        contentType: response.response.headers.get('content-type') ?? 'application/octet-stream',
        base64: Buffer.from(await response.data.arrayBuffer()).toString('base64'),
      },
      problem: null,
    }
  })

export const uploadAttachment = analystPost
  .validator((input: unknown) => {
    if (!(input instanceof FormData)) throw new Error('expected a form')
    const file = input.get('file')
    const disputeId = input.get('disputeId')
    if (!(file instanceof File) || typeof disputeId !== 'string')
      throw new Error('expected file and disputeId')
    return { file, disputeId: parse(uuid)(disputeId) }
  })
  .handler(
    ({ data: { file, disputeId }, context }): Promise<Outcome<components['schemas']['Attachment']>> =>
      asAnalyst(context.api, (api) => {
        const multipart = new FormData()
        multipart.append('file', file, file.name)
        return api.POST('/disputes/{disputeId}/attachments', {
          params: { path: { disputeId } },
          body: { file: file.name },
          bodySerializer: () => multipart,
        })
      }),
  )
