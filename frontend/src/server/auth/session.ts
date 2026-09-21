import { createServerFn } from '@tanstack/react-start'

import * as impl from './session-impl'

export type { Viewer } from './session-impl'

// Thin server functions over session-impl.ts, so route files can import this module without dragging server-only
// code into the browser bundle.
export const getViewer = createServerFn({ method: 'GET' }).handler(() => impl.viewer())

export const beginLogin = createServerFn({ method: 'POST' })
  .inputValidator((input: { slug: string; next?: string }) => input)
  .handler(({ data }) => impl.begin(data))

export const getAccessToken = createServerFn({ method: 'POST' }).handler(() => impl.accessToken())

export const logout = createServerFn({ method: 'POST' }).handler(() => impl.endSession())
