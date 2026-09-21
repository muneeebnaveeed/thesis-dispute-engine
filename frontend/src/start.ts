import { createMiddleware, createStart } from '@tanstack/react-start'

import { securityHeaders } from './server/security-headers'

// Every response, including server-function calls, passes through here.
const headers = createMiddleware({ type: 'request' }).server(async ({ next }) => {
  const nonce = crypto.randomUUID().replaceAll('-', '')
  const result = await next({ context: { nonce } })
  securityHeaders(result.response.headers, nonce)
  return result
})

export const startInstance = createStart(() => ({ requestMiddleware: [headers] }))
