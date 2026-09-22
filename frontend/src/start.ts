import { createMiddleware, createStart } from '@tanstack/react-start'

import { responseAdapter } from './api/response-adapter'
import type { getRouter } from './router'
import { securityHeaders } from './server/runtime/security-headers'
import { startTelemetry } from './server/telemetry/sdk'
import { traceRequest } from './server/telemetry/request'

// first in the chain: everything below it, server functions and API calls included, lands inside the request span
const withTracing = createMiddleware({ type: 'request' }).server(({ next, request }) => {
  startTelemetry()
  let result!: Awaited<ReturnType<typeof next>>
  return traceRequest(request, async () => {
    result = await next()
    return result.response
  }).then(() => result)
})

const withSecurityHeaders = createMiddleware({ type: 'request' }).server(async ({ next }) => {
  const nonce = crypto.randomUUID().replaceAll('-', '')
  const result = await next({ context: { nonce } })
  securityHeaders(result.response.headers, nonce)
  return result
})

export const startInstance = createStart(() => ({
  requestMiddleware: [withTracing, withSecurityHeaders],
  serializationAdapters: [responseAdapter],
}))

// the Vite plugin writes this block into routeTree.gen.ts, the tsr CLI (what CI runs before typecheck) does not;
// owning it here keeps the type check the same in both. All three members are needed: with `config` alone the
// serialization adapters are not picked up
declare module '@tanstack/react-start' {
  interface Register {
    ssr: true
    router: Awaited<ReturnType<typeof getRouter>>
    config: Awaited<ReturnType<typeof startInstance.getOptions>>
  }
}
