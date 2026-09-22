import { Type } from '@sinclair/typebox'

import { ApplyEventRequest, CreateDisputeRequest, DisputeState } from '#/api/schemas.gen'
import { withDisputeView } from '#/api/views'
import { analystGet, analystPost, asAnalyst, parse, uuid } from '#/server/runtime/fn'

export const DisputeListSearch = Type.Object({
  state: Type.Optional(DisputeState),
  cursor: Type.Optional(Type.String()),
  overdue: Type.Optional(Type.Boolean()),
})

export const getDispute = analystGet
  .validator(parse(uuid))
  .handler(
    asAnalyst((api, disputeId) =>
      withDisputeView(api.GET('/disputes/{disputeId}', { params: { path: { disputeId } } })),
    ),
  )

export const listDisputes = analystGet
  .validator(parse(DisputeListSearch))
  .handler(asAnalyst((api, search) => api.GET('/disputes', { params: { query: search } })))

export const openDispute = analystPost
  .validator(parse(CreateDisputeRequest))
  .handler(
    asAnalyst((api, request) =>
      withDisputeView(
        api.POST('/disputes', { body: request, headers: { 'Idempotency-Key': crypto.randomUUID() } }),
      ),
    ),
  )

export const applyEvent = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ApplyEventRequest })))
  .handler(
    asAnalyst((api, { disputeId, body }) =>
      withDisputeView(
        api.POST('/disputes/{disputeId}/events', {
          params: { path: { disputeId } },
          body,
          headers: { 'Idempotency-Key': crypto.randomUUID() },
        }),
      ),
    ),
  )
