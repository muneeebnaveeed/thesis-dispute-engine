import { FormatRegistry, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'

// neither side validates uuid by default; the backend registers the same pattern (RFC 4122 layout, any version)
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
if (!FormatRegistry.Has('uuid')) FormatRegistry.Set('uuid', (value) => uuidPattern.test(value))
// only catches a missing @ or domain; deliverability is the relay's problem
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
if (!FormatRegistry.Has('email')) FormatRegistry.Set('email', (value) => emailPattern.test(value))

export type Validation<T> = { ok: true; value: T } | { ok: false; errors: Record<string, string> }

export const validate = <T extends TSchema>(schema: T, input: unknown): Validation<Static<T>> => {
  const value = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
  if (Value.Check(schema, value)) return { ok: true, value }
  const errors: Record<string, string> = {}
  for (const schemaError of Value.Errors(schema, value)) {
    const fieldPath = schemaError.path.replace(/^\//, '') || '_'
    errors[fieldPath] ??= schemaError.message
  }
  return { ok: false, errors }
}
