import { Type } from '@sinclair/typebox'

import { ComposeEmailRequest } from '#/api/schemas.gen'
import { withDisputeView } from '#/api/views'
import { authenticatedGet, authenticatedPost, parse, uuid } from '#/server/runtime/fn'

const NoticeRef = Type.Object({ disputeId: uuid, noticeId: Type.Integer({ minimum: 1 }) })
const AttachmentRef = Type.Object({ disputeId: uuid, attachmentId: uuid })

type AttachmentBytes = { contentType: string; base64: string }
type MultipartUpload = { file: File; disputeId: string }

export const getNotice = authenticatedGet
  .validator(parse(NoticeRef))
  .handler(({ data: noticeRef, context: { api } }) =>
    api.GET('/disputes/{disputeId}/notices/{noticeId}', { params: { path: noticeRef } }),
  )

export const listEmailTemplates = authenticatedGet
  .validator(parse(uuid))
  .handler(({ data: disputeId, context: { api } }) =>
    api.GET('/disputes/{disputeId}/email-templates', { params: { path: { disputeId } } }),
  )

export const composeEmail = authenticatedPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ComposeEmailRequest })))
  .handler(({ data: { disputeId, body }, context: { api } }) =>
    withDisputeView(api.POST('/disputes/{disputeId}/notices', { params: { path: { disputeId } }, body })),
  )

export const resendNotice = authenticatedPost
  .validator(parse(NoticeRef))
  .handler(({ data: noticeRef, context: { api } }) =>
    withDisputeView(
      api.POST('/disputes/{disputeId}/notices/{noticeId}/resend', { params: { path: noticeRef } }),
    ),
  )

export const getAttachment = authenticatedGet
  .validator(parse(AttachmentRef))
  .handler(async ({ data: attachmentRef, context: { api } }) => {
    const result = await api.GET('/disputes/{disputeId}/attachments/{attachmentId}', {
      params: { path: attachmentRef },
      parseAs: 'blob',
    })
    if (result.error) return result
    const bytes: AttachmentBytes = {
      contentType: result.response.headers.get('content-type') ?? 'application/octet-stream',
      base64: Buffer.from(await result.data.arrayBuffer()).toString('base64'),
    }
    return { data: bytes, response: result.response }
  })

// the browser sends a FormData, which no TypeBox schema describes
const multipartUpload = (input: unknown): MultipartUpload => {
  if (!(input instanceof FormData)) throw new Error('expected a form')
  const file = input.get('file')
  const disputeId = input.get('disputeId')
  if (!(file instanceof File) || typeof disputeId !== 'string') throw new Error('expected file and disputeId')
  return { file, disputeId: parse(uuid)(disputeId) }
}

export const uploadAttachment = authenticatedPost
  .validator(multipartUpload)
  .handler(({ data: { file, disputeId }, context: { api } }) => {
    const multipart = new FormData()
    multipart.append('file', file, file.name)
    return api.POST('/disputes/{disputeId}/attachments', {
      params: { path: { disputeId } },
      body: { file: file.name },
      bodySerializer: () => multipart,
    })
  })
