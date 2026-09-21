import { createServerFn } from '@tanstack/react-start'

import { createApi } from '#/api/client'
import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { accessTokenForRequest } from './auth/session-impl'
import { unauthenticated } from './disputes-core'
import { serverEnv } from './env'

export type DisputePage = components['schemas']['DisputePage']
export type DisputeState = components['schemas']['DisputeState']

/** First paint of the workbench: the newest disputes, as the signed-in analyst. */
export const listDisputes = createServerFn({ method: 'GET' })
  .inputValidator(
    (input: { state?: DisputeState; cursor?: string; limit?: number; overdue?: boolean }) => input,
  )
  .handler(async ({ data }): Promise<{ value: DisputePage | null; problem: Problem | null }> => {
    const token = await accessTokenForRequest()
    if (!token) return { value: null, problem: unauthenticated().problem }
    const query: { state?: DisputeState; cursor?: string; limit?: number; overdue?: boolean } = {}
    if (data.state) query.state = data.state
    if (data.overdue) query.overdue = true
    if (data.cursor) query.cursor = data.cursor
    if (data.limit) query.limit = data.limit
    const { data: page, error } = await createApi(serverEnv().apiUrl, token).GET('/disputes', {
      params: { query },
    })
    return error ? { value: null, problem: error } : { value: page, problem: null }
  })
