import { Type } from '@sinclair/typebox'

import { NoticeKind, TemplateOverride } from '#/api/schemas.gen'
import { authenticatedGet, authenticatedPost, asOutcome, parse } from '#/server/runtime/fn'

export const listTenantTemplates = authenticatedGet.handler(asOutcome((api) => api.GET('/tenant-templates')))

export const putTenantTemplate = authenticatedPost
  .validator(parse(Type.Object({ kind: NoticeKind, override: TemplateOverride })))
  .handler(
    asOutcome((api, { kind, override }) =>
      api.PUT('/tenant-templates/{kind}', { params: { path: { kind } }, body: override }),
    ),
  )

export const deleteTenantTemplate = authenticatedPost
  .validator(parse(NoticeKind))
  .handler(asOutcome((api, kind) => api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } })))
