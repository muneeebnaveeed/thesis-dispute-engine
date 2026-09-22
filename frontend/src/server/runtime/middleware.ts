import { createMiddleware } from '@tanstack/react-start'

import { accessTokenForRequest } from '#/server/auth/session-impl'
import { createServerApi } from '#/server/runtime/api'

/**
 * Server functions that read on the analyst's behalf declare this middleware and receive `context.api`, a client
 * bound to the session's access token, or null when nobody is signed in. Resolving the session once here keeps
 * every read to its one line.
 */
export const authed = createMiddleware({ type: 'function' }).server(async ({ next }) => {
  const token = await accessTokenForRequest()
  return next({ context: { api: token ? createServerApi(token) : null } })
})
