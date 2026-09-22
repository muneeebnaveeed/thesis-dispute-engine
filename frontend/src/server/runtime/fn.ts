import { Type, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import { createServerFn } from '@tanstack/react-start'

import { traced } from '#/server/telemetry/server-fn'
import { authed } from './middleware'

export const uuid = Type.String({ format: 'uuid' })

// typed as Static<T> so call sites are checked; the RPC boundary still validates whatever arrives
export const parse =
  <T extends TSchema>(schema: T) =>
  (input: Static<T>): Static<T> => {
    const value = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
    if (!Value.Check(schema, value)) {
      const firstError = Value.Errors(schema, value).First()
      throw new Error(`invalid input${firstError ? ` at ${firstError.path}: ${firstError.message}` : ''}`)
    }
    return value
  }

export const publicGet = createServerFn({ method: 'GET' }).middleware([traced])
export const publicPost = createServerFn({ method: 'POST' }).middleware([traced])
export const authenticatedGet = createServerFn({ method: 'GET' }).middleware([traced, authed])
export const authenticatedPost = createServerFn({ method: 'POST' }).middleware([traced, authed])
