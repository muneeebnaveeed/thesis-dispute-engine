import { Type } from '@sinclair/typebox'

import type { components } from '#/api/schema.gen'
import { NoticeKind, TemplateOverride } from '#/api/schemas.gen'
import type { Outcome } from '#/api/views'
import { analystGet, analystPost, parse, asAnalyst } from '#/server/runtime/fn'

export const listTenantTemplates = analystGet.handler(
  ({ context }): Promise<Outcome<components['schemas']['TemplateSetting'][]>> =>
    asAnalyst(context.api, (api) => api.GET('/tenant-templates')),
)

export const putTenantTemplate = analystPost
  .validator(parse(Type.Object({ kind: NoticeKind, override: TemplateOverride })))
  .handler(
    ({ data: { kind, override }, context }): Promise<Outcome<components['schemas']['TemplateSetting']>> =>
      asAnalyst(context.api, (api) =>
        api.PUT('/tenant-templates/{kind}', { params: { path: { kind } }, body: override }),
      ),
  )

export const deleteTenantTemplate = analystPost
  .validator(parse(NoticeKind))
  .handler(
    ({ data: kind, context }): Promise<Outcome<undefined>> =>
      asAnalyst(context.api, (api) => api.DELETE('/tenant-templates/{kind}', { params: { path: { kind } } })),
  )
