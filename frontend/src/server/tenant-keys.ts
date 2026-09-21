import { createServerFn } from '@tanstack/react-start'

import { createApi } from '#/api/client'
import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { accessTokenForRequest } from './auth/session-impl'
import { unauthenticated } from './disputes-core'
import { serverEnv } from './env'

type TenantKey = components['schemas']['TenantKey']

/** First paint of the keys page: the list, as the signed-in analyst. */
export const listTenantKeys = createServerFn({ method: 'GET' }).handler(
  async (): Promise<{ value: TenantKey[] | null; problem: Problem | null }> => {
    const token = await accessTokenForRequest()
    if (!token) {
      const u = unauthenticated()
      return { value: null, problem: u.problem }
    }
    const { data, error } = await createApi(serverEnv().apiUrl, token).GET('/tenant-keys')
    return error ? { value: null, problem: error } : { value: data, problem: null }
  },
)
