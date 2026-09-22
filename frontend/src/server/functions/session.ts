import { Type } from '@sinclair/typebox'

import * as sessions from '#/server/auth/session-impl'
import { parse, publicGet, publicPost } from '#/server/runtime/fn'

// route files import this, never session-impl.ts, so server-only code stays out of the browser bundle
export const getViewer = publicGet.handler(() => sessions.viewer())

export const beginLogin = publicPost
  .validator(
    parse(
      Type.Object({
        slug: Type.String({ pattern: '^[a-z0-9-]{1,63}$' }),
        next: Type.Optional(Type.String({ maxLength: 2048 })),
      }),
    ),
  )
  .handler(({ data }) => sessions.begin(data))

export const logout = publicPost.handler(() => sessions.endSession())
