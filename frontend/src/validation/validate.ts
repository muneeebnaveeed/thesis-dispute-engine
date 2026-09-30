import type { Static, TSchema } from '@sinclair/typebox'
import type { StandardSchemaV1 } from '@standard-schema/spec'

import { fromTypeBox } from './typebox'

// the only place that knows which libraries define schemas; everything else validates through this module
export type Schema = StandardSchemaV1 | TSchema
export type Input<S extends Schema> = S extends StandardSchemaV1
  ? StandardSchemaV1.InferInput<S>
  : S extends TSchema
    ? Static<S>
    : never
export type Output<S extends Schema> = S extends StandardSchemaV1
  ? StandardSchemaV1.InferOutput<S>
  : S extends TSchema
    ? Static<S>
    : never

export type Validation<T> = { ok: true; value: T } | { ok: false; errors: Record<string, string> }

// TSchema's index signature defeats an 'in' narrowing
const isStandard = (schema: Schema): schema is StandardSchemaV1 => '~standard' in schema
const standard = (schema: Schema): StandardSchemaV1 => (isStandard(schema) ? schema : fromTypeBox(schema))

// dotted like form field names and the API's error pointers (answers.<id>); '_' is the value itself
const fieldName = (path: StandardSchemaV1.Issue['path']): string =>
  path?.map((segment) => String(typeof segment === 'object' ? segment.key : segment)).join('.') || '_'

export const validate = <S extends Schema>(schema: S, input: unknown): Validation<Output<S>> => {
  const result = standard(schema)['~standard'].validate(input)
  // forms and server-function validators call this synchronously
  if (result instanceof Promise) throw new TypeError('asynchronous schemas are not supported')
  // eslint-disable-next-line typescript/no-unsafe-type-assertion -- the adapter erases S; its output is S's by construction
  if (!result.issues) return { ok: true, value: result.value as Output<S> }
  const errors: Record<string, string> = {}
  for (const issue of result.issues) errors[fieldName(issue.path)] ??= issue.message
  return { ok: false, errors }
}

export const is = <S extends Schema>(schema: S, value: unknown): value is Output<S> =>
  validate(schema, value).ok
