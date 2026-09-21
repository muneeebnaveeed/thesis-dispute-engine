import { createServerFn } from '@tanstack/react-start'

import { createApi } from '#/api/client'
import type { Problem } from '#/api/failure'
import type { components } from '#/api/schema.gen'
import { accessTokenForRequest } from './auth/session-impl'
import { unauthenticated } from './disputes-core'
import { serverEnv } from './env'

export type NoticeDocument = components['schemas']['NoticeDocument']

/** One composed communication, for the letter page; rendered server-side so the letter prints without scripts. */
export const getNotice = createServerFn({ method: 'GET' })
  .inputValidator((input: { disputeId: string; noticeId: number }) => input)
  .handler(async ({ data }): Promise<{ value: NoticeDocument | null; problem: Problem | null }> => {
    const token = await accessTokenForRequest()
    if (!token) return { value: null, problem: unauthenticated().problem }
    const { data: doc, error } = await createApi(serverEnv().apiUrl, token).GET(
      '/disputes/{disputeId}/notices/{noticeId}',
      {
        params: { path: { disputeId: data.disputeId, noticeId: data.noticeId } },
      },
    )
    return error ? { value: null, problem: error } : { value: doc, problem: null }
  })
