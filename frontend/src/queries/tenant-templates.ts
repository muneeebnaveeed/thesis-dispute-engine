import { queryOptions } from '@tanstack/react-query'

import { listTenantTemplates } from '#/server/functions/tenant-templates'
import { classified } from './loaded'

export const tenantTemplatesQuery = () =>
  queryOptions({
    queryKey: ['tenant-templates'] as const,
    queryFn: () => listTenantTemplates(),
    select: classified,
  })
