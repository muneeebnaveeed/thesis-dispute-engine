import createClient, { type Middleware } from 'openapi-fetch'

import type { paths } from './schema.gen'

/** Server-side only: holds the tenant key. Never import from browser code. */
export function createApi(baseUrl: string, tenantKey: string) {
  const auth: Middleware = {
    onRequest({ request }) {
      request.headers.set('Authorization', `Bearer ${tenantKey}`)
      return request
    },
  }
  const client = createClient<paths>({ baseUrl })
  client.use(auth)
  return client
}

export type Api = ReturnType<typeof createApi>
