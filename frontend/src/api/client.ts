import createClient, { type Middleware } from 'openapi-fetch'

import type { paths } from './schema.gen'

export const createApi = (baseUrl: string, bearer: string) => {
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
