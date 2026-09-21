import type { paths } from '#/api/schema.gen'
import { type Static, Type } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import createClient from 'openapi-fetch'

import { serverEnv } from '../env'
import { open, seal } from './crypto'

/** The session payload. It never leaves this process in clear text; the API holds only the ciphertext. */
const SessionSchema = Type.Object({
  tenantSlug: Type.String(),
  pending: Type.Optional(
    Type.Object({ state: Type.String(), verifier: Type.String(), nonce: Type.String(), next: Type.String() }),
  ),
  identity: Type.Optional(
    Type.Object({
      tenantId: Type.String(),
      subject: Type.String(),
      name: Type.Union([Type.String(), Type.Null()]),
      email: Type.Union([Type.String(), Type.Null()]),
      roles: Type.Array(Type.String()),
    }),
  ),
  tokens: Type.Optional(
    Type.Object({
      access: Type.String(),
      refresh: Type.Union([Type.String(), Type.Null()]),
      id: Type.Union([Type.String(), Type.Null()]),
      accessExpiresAt: Type.String(),
    }),
  ),
})
export type Session = Static<typeof SessionSchema>

// The internal endpoints take the service key instead of a bearer credential.
function internal() {
  const env = serverEnv()
  const client = createClient<paths>({ baseUrl: env.apiUrl })
  client.use({
    onRequest({ request }) {
      request.headers.set('X-Service-Key', env.serviceKey)
      return request
    },
  })
  return client
}

export async function put(id: string, session: Session, ttlSeconds: number): Promise<void> {
  const body = {
    ciphertext: Buffer.from(await seal(session)).toString('base64'),
    expiresAt: new Date(Date.now() + ttlSeconds * 1000).toISOString(),
    ...(session.identity ? { tenantId: session.identity.tenantId } : {}),
  }
  const { response } = await internal().PUT('/internal/sessions/{sessionId}', {
    params: { path: { sessionId: id } },
    body,
  })
  if (!response.ok) throw new Error(`session store: put ${response.status}`)
}

export async function get(id: string): Promise<Session | null> {
  const { data, response } = await internal().GET('/internal/sessions/{sessionId}', {
    params: { path: { sessionId: id } },
  })
  if (response.status === 404) return null
  if (!response.ok || !data) throw new Error(`session store: get ${response.status}`)
  try {
    return asSession(await open(Buffer.from(data.ciphertext, 'base64')))
  } catch {
    // A payload sealed with a previous SESSION_SECRET is unreadable; treat it as signed out.
    return null
  }
}

export async function remove(id: string): Promise<void> {
  await internal().DELETE('/internal/sessions/{sessionId}', { params: { path: { sessionId: id } } })
}

// Shape check on what we ourselves sealed; a payload from an older build that no longer fits is treated as no session.
function asSession(v: unknown): Session | null {
  return Value.Check(SessionSchema, v) ? v : null
}
