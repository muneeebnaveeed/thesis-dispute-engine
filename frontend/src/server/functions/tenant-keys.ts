import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import { authenticatedGet, authenticatedPost, parse, uuid } from '#/server/runtime/fn'

export const listTenantKeys = authenticatedGet.handler(({ context: { api } }) => api.GET('/tenant-keys'))

export const issueTenantKey = authenticatedPost
  .validator(parse(CreateTenantKeyRequest))
  .handler(({ data: request, context: { api } }) => api.POST('/tenant-keys', { body: request }))

export const revokeTenantKey = authenticatedPost
  .validator(parse(uuid))
  .handler(({ data: keyId, context: { api } }) =>
    api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId } } }),
  )
