import { queryOptions } from '@tanstack/react-query'

import { getNotice, listEmailTemplates } from '#/server/functions/notices'
import { keyArgument } from './key-argument'
import { classified } from './loaded'

export const noticeQuery = (disputeId: string, noticeId: number | null) =>
  queryOptions({
    queryKey:
      noticeId === null ? (['notices', disputeId] as const) : (['notices', disputeId, noticeId] as const),
    queryFn: () => getNotice({ data: { disputeId, noticeId: keyArgument(noticeId, 'notice id') } }),
    select: classified,
  })

export const emailTemplatesQuery = (disputeId: string) =>
  queryOptions({
    queryKey: ['email-templates', disputeId] as const,
    queryFn: () => listEmailTemplates({ data: disputeId }),
    select: classified,
  })
