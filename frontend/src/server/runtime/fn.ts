import { Type, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import { createServerFn } from '@tanstack/react-start'

import type { Api } from '#/api/client'
import type { Problem } from '#/api/failure'
import { outcome, unauthenticated, type Outcome } from '#/api/views'
import { authed } from './middleware'

/**
 * The two bases every server function starts from: the analyst's session is resolved once by the middleware and
 * arrives as `context.api`. A builder is immutable, so sharing these is safe.
 */
export const analystGet = createServerFn({ method: 'GET' }).middleware([authed])
export const analystPost = createServerFn({ method: 'POST' }).middleware([authed])

// Shared pieces of every server function: input parsing with the contract's TypeBox schemas, and one way to turn
// an API result into the outcome pages render.

export const uuid = Type.String({ format: 'uuid' })

/** A validator for createServerFn: the input must satisfy the schema or the call is refused before any handler runs. */
export const parse =
  <T extends TSchema>(schema: T) =>
  (input: unknown): Static<T> => {
    const value = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
    if (!Value.Check(schema, value)) {
      const first = Value.Errors(schema, value).First()
      throw new Error(`invalid input${first ? ` at ${first.path}: ${first.message}` : ''}`)
    }
    return value
  }

/** Runs a call as the analyst, or answers unauthenticated when there is no session to run it as. */
type Read = {
  <T>(api: Api | null, run: (api: Api) => Promise<{ data?: T; error?: Problem }>): Promise<Outcome<T>>
  <T, V>(
    api: Api | null,
    run: (api: Api) => Promise<{ data?: T; error?: Problem }>,
    map: (t: T) => V,
  ): Promise<Outcome<V>>
}
export const read: Read = async <T, V>(
  api: Api | null,
  run: (api: Api) => Promise<{ data?: T; error?: Problem }>,
  map?: (t: T) => V,
): Promise<Outcome<T | V>> => {
  if (!api) return unauthenticated<T | V>()
  const res = await run(api)
  return map ? outcome(res, map) : outcome(res)
}
