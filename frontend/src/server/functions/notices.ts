import { Type } from '@sinclair/typebox'

import { ComposeEmailRequest } from '#/api/schemas.gen'
import { withDisputeView } from '#/api/views'
import { authenticatedGet, authenticatedPost, asOutcome, parse, uuid } from '#/server/runtime/fn'

const NoticeRef = Type.Object({ disputeId: uuid, noticeId: Type.Integer({ minimum: 1 }) })
const AttachmentRef = Type.Object({ disputeId: uuid, attachmentId: uuid })

type AttachmentBytes = { contentType: string; base64: string }
type MultipartUpload = { file: File; disputeId: string }

export const getNotice = authenticatedGet
  .validator(parse(NoticeRef))
  .handler(
    asOutcome((api, noticeRef) =>
      api.GET('/disputes/{disputeId}/notices/{noticeId}', { params: { path: noticeRef } }),
    ),
  )

export const listEmailTemplates = authenticatedGet
  .validator(parse(uuid))
  .handler(
    asOutcome((api, disputeId) =>
      api.GET('/disputes/{disputeId}/email-templates', { params: { path: { disputeId } } }),
    ),
  )

export const composeEmail = authenticatedPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ComposeEmailRequest })))
  .handler(
    asOutcome((api, { disputeId, body }) =>
      withDisputeView(api.POST('/disputes/{disputeId}/notices', { params: { path: { disputeId } }, body })),
    ),
  )

export const resendNotice = authenticatedPost
  .validator(parse(NoticeRef))
  .handler(
    asOutcome((api, noticeRef) =>
      withDisputeView(
        api.POST('/disputes/{disputeId}/notices/{noticeId}/resend', { params: { path: noticeRef } }),
      ),
    ),
  )

export const getAttachment = authenticatedGet.validator(parse(AttachmentRef)).handler(
  asOutcome(async (api, attachmentRef) => {
    const response = await api.GET('/disputes/{disputeId}/attachments/{attachmentId}', {
      params: { path: attachmentRef },
      parseAs: 'blob',
    })
    if (response.error) return { error: response.error }
    if (!response.data) return {}
    const bytes: AttachmentBytes = {
      contentType: response.response.headers.get('content-type') ?? 'application/octet-stream',
      base64: Buffer.from(await response.data.arrayBuffer()).toString('base64'),
    }
    return { data: bytes }
  }),
)

// the browser sends a FormData, which no TypeBox schema describes
const multipartUpload = (input: unknown): MultipartUpload => {
  if (!(input instanceof FormData)) throw new Error('expected a form')
  const file = input.get('file')
  const disputeId = input.get('disputeId')
  if (!(file instanceof File) || typeof disputeId !== 'string') throw new Error('expected file and disputeId')
  return { file, disputeId: parse(uuid)(disputeId) }
}

export const uploadAttachment = authenticatedPost.validator(multipartUpload).handler(
  asOutcome((api, { file, disputeId }) => {
    const multipart = new FormData()
    multipart.append('file', file, file.name)
    return api.POST('/disputes/{disputeId}/attachments', {
      params: { path: { disputeId } },
      body: { file: file.name },
      bodySerializer: () => multipart,
    })
  }),
)
