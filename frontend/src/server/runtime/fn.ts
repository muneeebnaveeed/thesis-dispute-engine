import { Type } from '@sinclair/typebox'
import { createServerFn } from '@tanstack/react-start'

import { validate, type Input, type Output, type Schema } from '#/validation/validate'

import { traced } from '#/server/telemetry/server-fn'
import { authed } from './middleware'

export const uuid = Type.String({ format: 'uuid' })

// typed as the schema's input so call sites are checked; the RPC boundary still validates whatever arrives
export const parse =
  <S extends Schema>(schema: S) =>
  (input: Input<S>): Output<S> => {
    const result = validate(schema, input)
    if (result.ok) return result.value
    const [field, message] = Object.entries(result.errors)[0] ?? []
    throw new Error(`invalid input${field ? ` at ${field}: ${message}` : ''}`)
  }

export const publicGet = createServerFn({ method: 'GET' }).middleware([traced])
export const publicPost = createServerFn({ method: 'POST' }).middleware([traced])
export const authenticatedGet = createServerFn({ method: 'GET' }).middleware([traced, authed])
export const authenticatedPost = createServerFn({ method: 'POST' }).middleware([traced, authed])
