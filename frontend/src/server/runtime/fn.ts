import { Type, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import { createServerFn } from '@tanstack/react-start'

import type { Api } from '#/api/client'
import type { Problem } from '#/api/failure'
import { toOutcome, unauthenticated, type Outcome } from '#/api/views'
import { authed } from './middleware'

export const analystGet = createServerFn({ method: 'GET' }).middleware([authed])
export const analystPost = createServerFn({ method: 'POST' }).middleware([authed])

export const uuid = Type.String({ format: 'uuid' })

export const parse =
  <T extends TSchema>(schema: T) =>
  (input: unknown): Static<T> => {
    const value = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
    if (!Value.Check(schema, value)) {
      const firstError = Value.Errors(schema, value).First()
      throw new Error(`invalid input${firstError ? ` at ${firstError.path}: ${firstError.message}` : ''}`)
    }
    return value
  }

type ApiCall<T> = (api: Api) => Promise<{ data?: T; error?: Problem }>
type AsAnalyst = {
  <T>(api: Api | null, call: ApiCall<T>): Promise<Outcome<T>>
  <T, V>(api: Api | null, call: ApiCall<T>, map: (value: T) => V): Promise<Outcome<V>>
}
// api is null when nobody is signed in; the call never runs and the page sees the same 401 the API would send
export const asAnalyst: AsAnalyst = async <T, V>(
  api: Api | null,
  call: ApiCall<T>,
  map?: (value: T) => V,
): Promise<Outcome<T | V>> => {
  if (!api) return unauthenticated<T | V>()
  const result = await call(api)
  return map ? toOutcome(result, map) : toOutcome(result)
}
