import { createServerFn } from '@tanstack/react-start'

import { createApi } from '#/api/client'
import { accessTokenForRequest } from './auth/session-impl'
import { applyDisputeEvent, fetchDispute, openDispute, unauthenticated } from './disputes-core'
import { serverEnv } from './env'

export type { Dispute, Json, Outcome } from './disputes-core'

// SSR and server-side actions call the API as the signed-in analyst; without a session they get a 401 problem,
// the same shape the API would return, so pages have one path.
async function api() {
  const token = await accessTokenForRequest()
  if (!token) return null
  return createApi(serverEnv().apiUrl, token)
}

export const getDispute = createServerFn({ method: 'GET' })
  .inputValidator((id: string) => id)
  .handler(async ({ data: id }) => {
    const a = await api()
    return a ? fetchDispute(a, id) : unauthenticated()
  })

export const createDispute = createServerFn({ method: 'POST' })
  .inputValidator((input: unknown) => input)
  .handler(async ({ data: input }) => {
    const a = await api()
    return a ? openDispute(a, input) : unauthenticated()
  })

export const applyEvent = createServerFn({ method: 'POST' })
  .inputValidator((input: { disputeId: string; body: unknown }) => input)
  .handler(async ({ data: { disputeId, body } }) => {
    const a = await api()
    return a ? applyDisputeEvent(a, disputeId, body) : unauthenticated()
  })
