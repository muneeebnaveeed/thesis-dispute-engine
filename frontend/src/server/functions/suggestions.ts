import { Type } from '@sinclair/typebox'

import { authenticatedPost, parse } from '#/server/runtime/fn'

// A suggestion is a convenience: the API answers with nothing when the model is unsure, unreachable or not
// configured, so the caller renders whatever it already had.
export const suggestSearchFilters = authenticatedPost
  .validator(parse(Type.Object({ query: Type.String({ minLength: 1, maxLength: 300 }) })))
  .handler(({ data: body, context: { api } }) => api.POST('/suggestions/search-filters', { body }))
