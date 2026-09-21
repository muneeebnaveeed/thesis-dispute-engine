import createClient, { type Middleware } from 'openapi-fetch'

import { getAccessToken } from '#/server/auth/session'
import type { paths } from './schema.gen'

// Direct browser-to-API calls (ADR 0011). The access token lives only in this module's memory; the server refreshes
// it, this side just asks again when it is about to expire or the API says 401.
let cached: { token: string; expiresAt: number } | null = null

async function token(force = false): Promise<string | null> {
  if (!force && cached && cached.expiresAt - Date.now() > 15_000) return cached.token
  const res = await getAccessToken()
  cached = res ? { token: res.token, expiresAt: Date.parse(res.expiresAt) } : null
  return cached?.token ?? null
}

/** Drops the remembered token, for example after logout. */
export function forgetToken() {
  cached = null
}

const bearer: Middleware = {
  async onRequest({ request }) {
    const t = await token()
    if (t) request.headers.set('Authorization', `Bearer ${t}`)
    return request
  },
  async onResponse({ request, response }) {
    // One retry with a fresh token covers a token that expired between our check and the API's.
    if (response.status !== 401 || request.headers.get('x-retried')) return response
    const t = await token(true)
    if (!t) return response
    const retry = new Request(request, { headers: new Headers(request.headers) })
    retry.headers.set('Authorization', `Bearer ${t}`)
    retry.headers.set('x-retried', '1')
    return fetch(retry)
  },
}

/** onSignedOut runs when no token can be obtained (the session ended); pages send the person back through sign-in. */
export function createBrowserApi(baseUrl: string, onSignedOut?: () => void) {
  const client = createClient<paths>({ baseUrl })
  client.use({
    async onRequest({ request }) {
      if (onSignedOut && !(await token())) onSignedOut()
      return request
    },
  })
  client.use(bearer)
  return client
}
