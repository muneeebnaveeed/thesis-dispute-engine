import { Type } from '@sinclair/typebox'

import type { components } from '#/api/schema.gen'
import {
  ApplyEventRequest,
  ComposeEmailRequest,
  CreateDisputeRequest,
  CreateTenantKeyRequest,
  NoticeKind,
  TemplateOverride,
} from '#/api/schemas.gen'
import { view, type Dispute, type Outcome } from '#/api/views'
import { analystPost, parse, read, uuid } from '#/server/runtime/fn'

// Every write the pages make, as the analyst. Each carries an Idempotency-Key minted here, so the API can
// repeat-proof it; each is validated against the contract's own schema before it leaves this process.

export const openDispute = analystPost
  .validator(parse(CreateDisputeRequest))
  .handler(
    ({ data, context }): Promise<Outcome<Dispute>> =>
      read(
        context.api,
        (api) => api.POST('/disputes', { body: data, headers: { 'Idempotency-Key': crypto.randomUUID() } }),
        view,
      ),
  )

export const applyEvent = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ApplyEventRequest })))
  .handler(
    ({ data, context }): Promise<Outcome<Dispute>> =>
      read(
        context.api,
        (api) =>
          api.POST('/disputes/{disputeId}/events', {
            params: { path: { disputeId: data.disputeId } },
            body: data.body,
            headers: { 'Idempotency-Key': crypto.randomUUID() },
          }),
        view,
      ),
  )

export const issueTenantKey = analystPost
  .validator(parse(CreateTenantKeyRequest))
  .handler(
    ({ data, context }): Promise<Outcome<components['schemas']['IssuedTenantKey']>> =>
      read(context.api, (api) => api.POST('/tenant-keys', { body: data })),
  )

export const revokeTenantKey = analystPost
  .validator(parse(uuid))
  .handler(
    ({ data: keyId, context }): Promise<Outcome<undefined>> =>
      read(context.api, (api) => api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId } } })),
  )

export const composeEmail = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ComposeEmailRequest })))
  .handler(
    ({ data, context }): Promise<Outcome<Dispute>> =>
      read(
        context.api,
        (api) =>
          api.POST('/disputes/{disputeId}/notices', {
            params: { path: { disputeId: data.disputeId } },
            body: data.body,
          }),
        view,
      ),
  )

export const resendNotice = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, noticeId: Type.Integer({ minimum: 1 }) })))
  .handler(
    ({ data, context }): Promise<Outcome<Dispute>> =>
      read(
        context.api,
        (api) => api.POST('/disputes/{disputeId}/notices/{noticeId}/resend', { params: { path: data } }),
        view,
      ),
  )

/** A multipart form with one `file` and the `disputeId`; the file is forwarded as received. */
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
    ({ data, context }): Promise<Outcome<components['schemas']['Attachment']>> =>
      read(context.api, (api) => {
        const form = new FormData()
        form.append('file', data.file, data.file.name)
        return api.POST('/disputes/{disputeId}/attachments', {
          params: { path: { disputeId: data.disputeId } },
          body: { file: data.file.name },
          bodySerializer: () => form,
        })
      }),
  )

export const putTenantTemplate = analystPost
  .validator(parse(Type.Object({ kind: NoticeKind, override: TemplateOverride })))
  .handler(
    ({ data, context }): Promise<Outcome<components['schemas']['TemplateSetting']>> =>
      read(context.api, (api) =>
        api.PUT('/tenant-templates/{kind}', { params: { path: { kind: data.kind } }, body: data.override }),
      ),
  )

export const deleteTenantTemplate = analystPost
  .validator(parse(NoticeKind))
  .handler(
    ({ data: kind, context }): Promise<Outcome<undefined>> =>
      read(context.api, (api) => api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } })),
  )
