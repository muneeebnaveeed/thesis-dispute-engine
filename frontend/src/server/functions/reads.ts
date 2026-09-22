import { Type } from '@sinclair/typebox'

import type { components } from '#/api/schema.gen'
import { DisputeState } from '#/api/schemas.gen'
import { view, type Dispute, type DisputePage, type Outcome } from '#/api/views'
import { analystGet, parse, read, uuid } from '#/server/runtime/fn'

// Every read the pages need, as the analyst, through the session held on the server (ADR 0020). On first paint
// these run in-process during SSR; after hydration the browser calls them as RPC. Nothing else reaches the API.

export const getDispute = analystGet
  .validator(parse(uuid))
  .handler(
    ({ data: id, context }): Promise<Outcome<Dispute>> =>
      read(
        context.api,
        (api) => api.GET('/disputes/{disputeId}', { params: { path: { disputeId: id } } }),
        view,
      ),
  )

export const ListSearch = Type.Object({
  state: Type.Optional(DisputeState),
  cursor: Type.Optional(Type.String()),
  overdue: Type.Optional(Type.Boolean()),
})

export const listDisputes = analystGet
  .validator(parse(ListSearch))
  .handler(
    ({ data, context }): Promise<Outcome<DisputePage>> =>
      read(context.api, (api) => api.GET('/disputes', { params: { query: data } })),
  )

export const listTenantKeys = analystGet.handler(
  ({ context }): Promise<Outcome<components['schemas']['TenantKey'][]>> =>
    read(context.api, (api) => api.GET('/tenant-keys')),
)

export const getNotice = analystGet
  .validator(parse(Type.Object({ disputeId: uuid, noticeId: Type.Integer({ minimum: 1 }) })))
  .handler(
    ({ data, context }): Promise<Outcome<components['schemas']['NoticeDocument']>> =>
      read(context.api, (api) =>
        api.GET('/disputes/{disputeId}/notices/{noticeId}', { params: { path: data } }),
      ),
  )

export const listEmailTemplates = analystGet
  .validator(parse(uuid))
  .handler(
    ({ data: disputeId, context }): Promise<Outcome<components['schemas']['EmailTemplates']>> =>
      read(context.api, (api) =>
        api.GET('/disputes/{disputeId}/email-templates', { params: { path: { disputeId } } }),
      ),
  )

export const listTenantTemplates = analystGet.handler(
  ({ context }): Promise<Outcome<components['schemas']['TemplateSetting'][]>> =>
    read(context.api, (api) => api.GET('/tenant-templates')),
)

/** An attachment's bytes, base64 for the wire; the browser turns it back into a file to save. */
export const getAttachment = analystGet
  .validator(parse(Type.Object({ disputeId: uuid, attachmentId: uuid })))
  .handler(async ({ data, context }): Promise<Outcome<{ contentType: string; base64: string }>> => {
    if (!context.api) return read(null, () => Promise.resolve({}))
    const res = await context.api.GET('/disputes/{disputeId}/attachments/{attachmentId}', {
      params: { path: data },
      parseAs: 'blob',
    })
    if (res.error || !res.data) return { value: null, problem: res.error ?? null }
    const bytes = Buffer.from(await res.data.arrayBuffer())
    return {
      value: {
        contentType: res.response.headers.get('content-type') ?? 'application/octet-stream',
        base64: bytes.toString('base64'),
      },
      problem: null,
    }
  })
