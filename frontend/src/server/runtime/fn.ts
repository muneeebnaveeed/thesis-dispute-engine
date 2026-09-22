import { Type, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'
import { createServerFn } from '@tanstack/react-start'

import type { Api } from '#/api/client'
import type { Problem } from '#/api/failure'
import { toOutcome, type Outcome } from '#/api/views'
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

type ApiResult<T> = { data?: T; error?: Problem }
type AnalystCall<Input, T> = (api: Api, input: Input) => Promise<ApiResult<T>>
type HandlerContext<Input> = { data: Input; context: { api: Api } }

export const analystGet = createServerFn({ method: 'GET' }).middleware([authed])
export const analystPost = createServerFn({ method: 'POST' }).middleware([authed])

// the handler of every analyst server function. Start's builder types cannot be wrapped generically, which is why
// this is a handler and not part of the builders above. Nothing server-only may be imported here: route files
// import this module and only middleware .server() bodies are stripped from the client bundle.
export const asAnalyst =
  <Input, T>(call: AnalystCall<Input, T>) =>
  ({ data, context }: HandlerContext<Input>): Promise<Outcome<T>> =>
    call(context.api, data).then(toOutcome)
