import { CreateTenantKeyRequest } from '#/api/schemas.gen'
import { authenticatedGet, authenticatedPost, asOutcome, parse, uuid } from '#/server/runtime/fn'

export const listTenantKeys = authenticatedGet.handler(asOutcome((api) => api.GET('/tenant-keys')))

export const issueTenantKey = authenticatedPost
  .validator(parse(CreateTenantKeyRequest))
  .handler(asOutcome((api, request) => api.POST('/tenant-keys', { body: request })))

export const revokeTenantKey = authenticatedPost
  .validator(parse(uuid))
  .handler(asOutcome((api, keyId) => api.DELETE('/tenant-keys/{keyId}', { params: { path: { keyId } } })))
