import { Type } from '@sinclair/typebox'

import { ApplyEventRequest, CreateDisputeRequest, DisputeState } from '#/api/schemas.gen'
import { disputeView, type Dispute, type DisputePage, type Outcome } from '#/api/views'
import { analystGet, analystPost, parse, asAnalyst, uuid } from '#/server/runtime/fn'

export const DisputeListSearch = Type.Object({
  state: Type.Optional(DisputeState),
  cursor: Type.Optional(Type.String()),
  overdue: Type.Optional(Type.Boolean()),
})

export const getDispute = analystGet
  .validator(parse(uuid))
  .handler(
    ({ data: disputeId, context }): Promise<Outcome<Dispute>> =>
      asAnalyst(
        context.api,
        (api) => api.GET('/disputes/{disputeId}', { params: { path: { disputeId } } }),
        disputeView,
      ),
  )

export const listDisputes = analystGet
  .validator(parse(DisputeListSearch))
  .handler(
    ({ data: search, context }): Promise<Outcome<DisputePage>> =>
      asAnalyst(context.api, (api) => api.GET('/disputes', { params: { query: search } })),
  )

export const openDispute = analystPost
  .validator(parse(CreateDisputeRequest))
  .handler(
    ({ data: request, context }): Promise<Outcome<Dispute>> =>
      asAnalyst(
        context.api,
        (api) =>
          api.POST('/disputes', { body: request, headers: { 'Idempotency-Key': crypto.randomUUID() } }),
        disputeView,
      ),
  )

export const applyEvent = analystPost
  .validator(parse(Type.Object({ disputeId: uuid, body: ApplyEventRequest })))
  .handler(
    ({ data: { disputeId, body }, context }): Promise<Outcome<Dispute>> =>
      asAnalyst(
        context.api,
        (api) =>
          api.POST('/disputes/{disputeId}/events', {
            params: { path: { disputeId } },
            body,
            headers: { 'Idempotency-Key': crypto.randomUUID() },
          }),
        disputeView,
      ),
  )
