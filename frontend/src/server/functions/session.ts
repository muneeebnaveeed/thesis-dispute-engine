import { Type } from '@sinclair/typebox'
import { createServerFn } from '@tanstack/react-start'

import * as sessions from '#/server/auth/session-impl'
import { parse } from '#/server/runtime/fn'

// route files import this, never session-impl.ts, so server-only code stays out of the browser bundle
export const getViewer = createServerFn({ method: 'GET' }).handler(() => sessions.viewer())

export const beginLogin = createServerFn({ method: 'POST' })
  .validator(
    parse(
      Type.Object({
        slug: Type.String({ pattern: '^[a-z0-9-]{1,63}$' }),
        next: Type.Optional(Type.String({ maxLength: 2048 })),
      }),
    ),
  )
  .handler(({ data }) => sessions.begin(data))

export const logout = createServerFn({ method: 'POST' }).handler(() => sessions.endSession())
