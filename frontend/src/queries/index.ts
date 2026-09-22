import { queryOptions } from '@tanstack/react-query'
import type { Static } from '@sinclair/typebox'

import {
  getDispute,
  getNotice,
  listDisputes,
  listEmailTemplates,
  listTenantKeys,
  listTenantTemplates,
  type ListSearch as ListSearchSchema,
} from '#/server/functions/reads'

/**
 * One query per read, fetched by a server function: in-process during SSR, where the result is dehydrated into
 * the page, and as RPC from the browser afterwards (ADR 0020). The options are the only source of key truth:
 * pass null for the identifying argument to get the prefix, so `disputeQuery(null).queryKey` is `['disputes']`
 * and invalidating it covers the list and every single dispute.
 */
export type ListSearch = Static<typeof ListSearchSchema>

const required = <T>(v: T | null, what: string): T => {
  if (v === null) throw new Error(`${what} is required to fetch; null is for the key prefix only`)
  return v
}

export const disputeQuery = (id: string | null) =>
  queryOptions({
    queryKey: id === null ? (['disputes'] as const) : (['disputes', id] as const),
    queryFn: () => getDispute({ data: required(id, 'dispute id') }),
  })

export const disputesQuery = (search: ListSearch | null) =>
  queryOptions({
    queryKey: search === null ? (['disputes'] as const) : (['disputes', { list: search }] as const),
    queryFn: () => listDisputes({ data: required(search, 'list search') }),
  })

export const tenantKeysQuery = () =>
  queryOptions({ queryKey: ['tenant-keys'] as const, queryFn: () => listTenantKeys() })

export const noticeQuery = (disputeId: string, noticeId: number | null) =>
  queryOptions({
    queryKey:
      noticeId === null ? (['notices', disputeId] as const) : (['notices', disputeId, noticeId] as const),
    queryFn: () => getNotice({ data: { disputeId, noticeId: required(noticeId, 'notice id') } }),
  })

export const emailTemplatesQuery = (disputeId: string) =>
  queryOptions({
    queryKey: ['email-templates', disputeId] as const,
    queryFn: () => listEmailTemplates({ data: disputeId }),
  })

export const tenantTemplatesQuery = () =>
  queryOptions({ queryKey: ['tenant-templates'] as const, queryFn: () => listTenantTemplates() })
