import createClient, { type Middleware } from 'openapi-fetch'

import type { paths } from './schema.gen'

/** A client bound to one bearer credential: an analyst access token here, a tenant key for machine callers. */
export function createApi(baseUrl: string, bearer: string) {
  const auth: Middleware = {
    onRequest({ request }) {
      request.headers.set('Authorization', `Bearer ${bearer}`)
      return request
    },
  }
  const client = createClient<paths>({ baseUrl })
  client.use(auth)
  return client
}

export type Api = ReturnType<typeof createApi>
