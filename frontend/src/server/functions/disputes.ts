import { Type } from '@sinclair/typebox'

import { ApplyEventRequest, CreateDisputeRequest, DisputeState } from '#/api/schemas.gen'
import { withDisputeView } from '#/api/views'
import { authenticatedGet, authenticatedPost, asOutcome, parse, uuid } from '#/server/runtime/fn'

export const DisputeListSearch = Type.Object({
  state: Type.Optional(DisputeState),
  cursor: Type.Optional(Type.String()),
  overdue: Type.Optional(Type.Boolean()),
})

export const getDispute = authenticatedGet
  .validator(parse(uuid))
  .handler(
    asOutcome((api, disputeId) =>
      withDisputeView(api.GET('/disputes/{disputeId}', { params: { path: { disputeId } } })),
    ),
  )

export const listDisputes = authenticatedGet
  .validator(parse(DisputeListSearch))
  .handler(asOutcome((api, search) => api.GET('/disputes', { params: { query: search } })))

export const openDispute = authenticatedPost
  .validator(parse(CreateDisputeRequest))
  .handler(
    asOutcome((api, request) =>
      withDisputeView(
        api.POST('/disputes', { body: request, headers: { 'Idempotency-Key': crypto.randomUUID() } }),
      ),
    ),
  )

export const applyEvent = authenticatedPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ApplyEventRequest })))
  .handler(
    asOutcome((api, { disputeId, body }) =>
      withDisputeView(
        api.POST('/disputes/{disputeId}/events', {
          params: { path: { disputeId } },
          body,
          headers: { 'Idempotency-Key': crypto.randomUUID() },
        }),
      ),
    ),
  )
