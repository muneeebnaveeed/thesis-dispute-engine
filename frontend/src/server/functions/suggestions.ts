import { Type } from '@sinclair/typebox'

import { authenticatedPost, parse, uuid } from '#/server/runtime/fn'

// A suggestion is a convenience: the API answers with nothing when the model is unsure, unreachable or not
// configured, so the caller renders whatever it already had.
export const suggestQuestionnaireAnswers = authenticatedPost
  .validator(parse(Type.Object({ disputeId: uuid, reply: Type.String({ minLength: 1, maxLength: 4000 }) })))
  .handler(({ data: { disputeId, reply }, context: { api } }) =>
    api.POST('/disputes/{disputeId}/suggestions/questionnaire', {
      params: { path: { disputeId } },
      body: { reply },
    }),
  )

export const suggestDisputeReason = authenticatedPost
  .validator(parse(Type.Object({ description: Type.String({ minLength: 1, maxLength: 2000 }) })))
  .handler(({ data: body, context: { api } }) => api.POST('/suggestions/dispute-reason', { body }))

export const suggestSearchFilters = authenticatedPost
  .validator(parse(Type.Object({ query: Type.String({ minLength: 1, maxLength: 300 }) })))
  .handler(({ data: body, context: { api } }) => api.POST('/suggestions/search-filters', { body }))
