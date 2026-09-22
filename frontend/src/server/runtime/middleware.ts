import { createMiddleware } from '@tanstack/react-start'

import { accessTokenForRequest } from '#/server/auth/session-impl'
import { createServerApi } from '#/server/runtime/api'
import { signInAgain } from '#/server/runtime/sign-in-again'

// an API 401 means the token died between the session check and the call; treated like a missing session
const isSignedOutOutcome = (handlerResult: unknown): boolean =>
  typeof handlerResult === 'object' &&
  handlerResult !== null &&
  'result' in handlerResult &&
  typeof handlerResult.result === 'object' &&
  handlerResult.result !== null &&
  'problem' in handlerResult.result &&
  typeof handlerResult.result.problem === 'object' &&
  handlerResult.result.problem !== null &&
  'code' in handlerResult.result.problem &&
  handlerResult.result.problem.code === 'unauthenticated'

export const authed = createMiddleware({ type: 'function' }).server(async ({ next }) => {
  const token = await accessTokenForRequest()
  if (!token) throw signInAgain()
  const handled = await next({ context: { api: createServerApi(token) } })
  if (isSignedOutOutcome(handled)) throw signInAgain()
  return handled
})
