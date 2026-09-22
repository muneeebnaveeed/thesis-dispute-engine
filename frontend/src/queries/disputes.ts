import { queryOptions } from '@tanstack/react-query'
import type { Static } from '@sinclair/typebox'

import { getDispute, listDisputes, type DisputeListSearch } from '#/server/functions/disputes'
import { keyArgument } from './key-argument'
import { classified } from './loaded'

export type DisputeListSearch = Static<typeof DisputeListSearch>

export const disputeQuery = (disputeId: string | null) =>
  queryOptions({
    queryKey: disputeId === null ? (['disputes'] as const) : (['disputes', disputeId] as const),
    queryFn: () => getDispute({ data: keyArgument(disputeId, 'dispute id') }),
    select: classified,
  })

export const disputesQuery = (search: DisputeListSearch | null) =>
  queryOptions({
    queryKey: search === null ? (['disputes'] as const) : (['disputes', { list: search }] as const),
    queryFn: () => listDisputes({ data: keyArgument(search, 'list search') }),
    select: classified,
  })
