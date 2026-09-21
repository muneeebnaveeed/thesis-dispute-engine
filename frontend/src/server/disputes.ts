import { createServerFn } from '@tanstack/react-start'

import { createApi } from '#/api/client'
import { applyDisputeEvent, fetchDispute, openDispute } from './disputes-core'
import { serverEnv } from './env'

export type { Dispute, Json, Outcome } from './disputes-core'

function api() {
  const env = serverEnv()
  return createApi(env.apiUrl, env.tenantKey)
}

export const getDispute = createServerFn({ method: 'GET' })
  .inputValidator((id: string) => id)
  .handler(({ data: id }) => fetchDispute(api(), id))

export const createDispute = createServerFn({ method: 'POST' })
  .inputValidator((input: unknown) => input)
  .handler(({ data: input }) => openDispute(api(), input))

export const applyEvent = createServerFn({ method: 'POST' })
  .inputValidator((input: { disputeId: string; body: unknown }) => input)
  .handler(({ data: { disputeId, body } }) => applyDisputeEvent(api(), disputeId, body))
