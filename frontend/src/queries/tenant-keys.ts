import { queryOptions } from '@tanstack/react-query'

import { listTenantKeys } from '#/server/functions/tenant-keys'
import { classified } from './loaded'

export const tenantKeysQuery = () =>
  queryOptions({ queryKey: ['tenant-keys'] as const, queryFn: () => listTenantKeys(), select: classified })
