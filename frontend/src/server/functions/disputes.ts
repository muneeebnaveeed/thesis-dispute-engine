import { Type } from '@sinclair/typebox'

import { ApplyEventRequest, CreateDisputeRequest, DisputeState } from '#/api/schemas.gen'
import { withSerialisableEvents } from '#/api/views'
import { authenticatedGet, authenticatedPost, parse, uuid } from '#/server/runtime/fn'

export const DisputeListSearch = Type.Object({
  state: Type.Optional(DisputeState),
  cursor: Type.Optional(Type.String()),
  overdue: Type.Optional(Type.Boolean()),
})

export const getDispute = authenticatedGet
  .validator(parse(uuid))
  .handler(({ data: disputeId, context: { api } }) =>
    withSerialisableEvents(api.GET('/disputes/{disputeId}', { params: { path: { disputeId } } })),
  )

export const listDisputes = authenticatedGet
  .validator(parse(DisputeListSearch))
  .handler(({ data: search, context: { api } }) => api.GET('/disputes', { params: { query: search } }))

export const openDispute = authenticatedPost
  .validator(parse(CreateDisputeRequest))
  .handler(({ data: request, context: { api } }) =>
    withSerialisableEvents(
      api.POST('/disputes', { body: request, headers: { 'Idempotency-Key': crypto.randomUUID() } }),
    ),
  )

export const applyEvent = authenticatedPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ApplyEventRequest })))
  .handler(({ data: { disputeId, body }, context: { api } }) =>
    withSerialisableEvents(
      api.POST('/disputes/{disputeId}/events', {
        params: { path: { disputeId } },
        body,
        headers: { 'Idempotency-Key': crypto.randomUUID() },
      }),
    ),
  )
