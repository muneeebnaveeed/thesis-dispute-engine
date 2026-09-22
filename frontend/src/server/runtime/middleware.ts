import { createMiddleware } from '@tanstack/react-start'

import { accessTokenForRequest } from '#/server/auth/session-impl'
import { createServerApi } from '#/server/runtime/api'
import { signInAgain } from '#/server/runtime/sign-in-again'

export const authed = createMiddleware({ type: 'function' }).server(async ({ next }) => {
  const token = await accessTokenForRequest()
  if (!token) throw signInAgain()
  return next({ context: { api: createServerApi(token) } })
})
