import { Type } from '@sinclair/typebox'

import { NoticeKind, TemplateOverride } from '#/api/schemas.gen'
import { authenticatedGet, authenticatedPost, parse } from '#/server/runtime/fn'

export const listTenantTemplates = authenticatedGet.handler(({ context: { api } }) =>
  api.GET('/tenant-templates'),
)

export const putTenantTemplate = authenticatedPost
  .validator(parse(Type.Object({ kind: NoticeKind, override: TemplateOverride })))
  .handler(({ data: { kind, override }, context: { api } }) =>
    api.PUT('/tenant-templates/{kind}', { params: { path: { kind } }, body: override }),
  )

export const deleteTenantTemplate = authenticatedPost
  .validator(parse(NoticeKind))
  .handler(({ data: kind, context: { api } }) =>
    api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } }),
  )
