import { queryOptions } from '@tanstack/react-query'

import { getBranding, getMyAvatar } from '#/server/functions/identity'
import { classified } from './loaded'

export const brandingQuery = () =>
  queryOptions({ queryKey: ['branding'] as const, queryFn: () => getBranding(), select: classified })

export const myAvatarQuery = () =>
  queryOptions({ queryKey: ['my-avatar'] as const, queryFn: () => getMyAvatar(), select: classified })
