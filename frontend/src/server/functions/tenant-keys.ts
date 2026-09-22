import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import { analystGet, analystPost, asAnalyst, parse, uuid } from '#/server/runtime/fn'

export const listTenantKeys = analystGet.handler(asAnalyst((api) => api.GET('/tenant-keys')))

export const issueTenantKey = analystPost
  .validator(parse(CreateTenantKeyRequest))
  .handler(asAnalyst((api, request) => api.POST('/tenant-keys', { body: request })))

export const revokeTenantKey = analystPost
  .validator(parse(uuid))
  .handler(asAnalyst((api, keyId) => api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId } } })))
