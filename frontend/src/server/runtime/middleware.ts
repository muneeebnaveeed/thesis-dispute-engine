import { createMiddleware } from '@tanstack/react-start'

import { accessTokenForRequest } from '#/server/auth/session-impl'
import { createServerApi } from '#/server/runtime/api'

export const authed = createMiddleware({ type: 'function' }).server(async ({ next }) => {
  const token = await accessTokenForRequest()
  return next({ context: { api: token ? createServerApi(token) : null } })
})
