import type { paths } from '#/api/schema.gen'
import { type Static, Type } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import createClient from 'openapi-fetch'

import { serverEnv } from '#/server/runtime/env'
import { open, seal } from './crypto'

const SessionSchema = Type.Object({
  tenantSlug: Type.String(),
  pending: Type.Optional(
    Type.Object({ state: Type.String(), verifier: Type.String(), nonce: Type.String(), next: Type.String() }),
  ),
  lastSeenAt: Type.Optional(Type.String()),
  identity: Type.Optional(
    Type.Object({
      tenantId: Type.String(),
      subject: Type.String(),
      sid: Type.Optional(Type.Union([Type.String(), Type.Null()])),
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

const internalApi = () => {
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

export const put = async (sessionId: string, session: Session, ttlSeconds: number): Promise<void> => {
  // sub and sid stay in the clear so a back-channel logout can find the rows
  const row = {
    ciphertext: Buffer.from(await seal(session)).toString('base64'),
    expiresAt: new Date(Date.now() + ttlSeconds * 1000).toISOString(),
    ...(session.identity ? { tenantId: session.identity.tenantId, subject: session.identity.subject } : {}),
    ...(session.identity?.sid ? { sid: session.identity.sid } : {}),
  }
  const { response } = await internalApi().PUT('/internal/sessions/{sessionId}', {
    params: { path: { sessionId } },
    body: row,
  })
  if (!response.ok) throw new Error(`session store: put ${response.status}`)
}

export const get = async (sessionId: string): Promise<Session | null> => {
  const { data: row, response } = await internalApi().GET('/internal/sessions/{sessionId}', {
    params: { path: { sessionId } },
  })
  if (response.status === 404) return null
  if (!response.ok || !row) throw new Error(`session store: get ${response.status}`)
  try {
    return asSession(await open(Buffer.from(row.ciphertext, 'base64')))
  } catch {
    // sealed under a previous SESSION_SECRET: signed out
    return null
  }
}

export const remove = async (sessionId: string): Promise<void> => {
  await internalApi().DELETE('/internal/sessions/{sessionId}', { params: { path: { sessionId } } })
}

// an older build's payload that no longer fits is no session
const asSession = (unsealed: unknown): Session | null =>
  Value.Check(SessionSchema, unsealed) ? unsealed : null
