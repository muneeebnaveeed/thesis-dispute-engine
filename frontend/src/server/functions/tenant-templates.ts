import { Type } from '@sinclair/typebox'

import { NoticeKind, TemplateOverride } from '#/api/schemas.gen'
import { analystGet, analystPost, asAnalyst, parse } from '#/server/runtime/fn'

export const listTenantTemplates = analystGet.handler(asAnalyst((api) => api.GET('/tenant-templates')))

export const putTenantTemplate = analystPost
  .validator(parse(Type.Object({ kind: NoticeKind, override: TemplateOverride })))
  .handler(
    asAnalyst((api, { kind, override }) =>
      api.PUT('/tenant-templates/{kind}', { params: { path: { kind } }, body: override }),
    ),
  )

export const deleteTenantTemplate = analystPost
  .validator(parse(NoticeKind))
  .handler(asAnalyst((api, kind) => api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } })))
