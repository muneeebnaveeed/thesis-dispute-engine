import { FormatRegistry, type Static, type TSchema } from '@sinclair/typebox'
import { Value } from '@sinclair/typebox/value'

// Neither side validates uuid by default; the backend registers the same pattern (RFC 4122 layout, any version).
const uuid = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
if (!FormatRegistry.Has('uuid')) FormatRegistry.Set('uuid', (v) => uuid.test(v))
// Enough of an email check to catch a missing @ or domain; deliverability is the relay's problem, not the form's.
const email = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
if (!FormatRegistry.Has('email')) FormatRegistry.Set('email', (v) => email.test(v))

export type Validation<T> = { ok: true; value: T } | { ok: false; errors: Record<string, string> }

/** Structural validation only; business rules come back from the API as problems. */
export const validate = <T extends TSchema>(schema: T, input: unknown): Validation<Static<T>> => {
  const value = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
  if (Value.Check(schema, value)) return { ok: true, value }
  const errors: Record<string, string> = {}
  for (const e of Value.Errors(schema, value)) {
    const key = e.path.replace(/^\//, '') || '_'
    errors[key] ??= e.message
  }
  return { ok: false, errors }
}
