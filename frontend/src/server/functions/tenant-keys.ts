import type { components } from '#/api/schema.gen'
import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import type { Outcome } from '#/api/views'
import { analystGet, analystPost, parse, asAnalyst, uuid } from '#/server/runtime/fn'

export const listTenantKeys = analystGet.handler(
  ({ context }): Promise<Outcome<components['schemas']['TenantKey'][]>> =>
    asAnalyst(context.api, (api) => api.GET('/tenant-keys')),
)

export const issueTenantKey = analystPost
  .validator(parse(CreateTenantKeyRequest))
  .handler(
    ({ data: request, context }): Promise<Outcome<components['schemas']['IssuedTenantKey']>> =>
      asAnalyst(context.api, (api) => api.POST('/tenant-keys', { body: request })),
  )

export const revokeTenantKey = analystPost
  .validator(parse(uuid))
  .handler(
    ({ data: keyId, context }): Promise<Outcome<undefined>> =>
      asAnalyst(context.api, (api) => api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId } } })),
  )
