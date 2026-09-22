import { Type } from '@sinclair/typebox'
import { createServerFn } from '@tanstack/react-start'

import { parse } from '#/server/runtime/fn'

import * as impl from '#/server/auth/session-impl'

export type { Viewer } from '#/server/auth/session-impl'

// Thin server functions over session-impl.ts, so route files can import this module without dragging server-only
// code into the browser bundle.
export const getViewer = createServerFn({ method: 'GET' }).handler(() => impl.viewer())

export const beginLogin = createServerFn({ method: 'POST' })
  .validator(
    parse(
      Type.Object({
        slug: Type.String({ pattern: '^[a-z0-9-]{1,63}$' }),
        next: Type.Optional(Type.String({ maxLength: 2048 })),
      }),
    ),
  )
  .handler(({ data }) => impl.begin(data))

export const logout = createServerFn({ method: 'POST' }).handler(() => impl.endSession())
