import { FormatRegistry, type Static, type TSchema } from '@sinclair/typebox'
import { Value, ValueErrorType, type ValueError } from '@sinclair/typebox/value'
import type { StandardSchemaV1 } from '@standard-schema/spec'

// neither side validates uuid by default; the backend registers the same pattern (RFC 4122 layout, any version)
const uuidPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i
if (!FormatRegistry.Has('uuid')) FormatRegistry.Set('uuid', (value) => uuidPattern.test(value))
// only catches a missing @ or domain; deliverability is the relay's problem
const emailPattern = /^[^\s@]+@[^\s@]+\.[^\s@]+$/
if (!FormatRegistry.Has('email')) FormatRegistry.Set('email', (value) => emailPattern.test(value))

// "transactionId" reads as "transaction ID" in a message
const spokenName = (fieldPath: string): string =>
  (fieldPath.split('/').at(-1) ?? fieldPath)
    .replace(/([a-z])([A-Z])/g, '$1 $2')
    .toLowerCase()
    .replace(/\bid\b/g, 'ID')

const chooses = (schema: TSchema): boolean => 'anyOf' in schema || 'enum' in schema

// messages are shown under the input, so they say what to do; the schema says only which rule failed
const messageFor = (error: ValueError): string => {
  const name = spokenName(error.path)
  const { schema, type } = error
  if (type === ValueErrorType.ObjectRequiredProperty || type === ValueErrorType.StringMinLength) {
    return chooses(schema) ? `Please select the ${name}` : `Please enter the ${name}`
  }
  if (type === ValueErrorType.Union) return `Please select the ${name}`
  if (type === ValueErrorType.StringFormat) {
    return schema.format === 'email' ? 'Please enter a valid email address' : `Please enter a valid ${name}`
  }
  if (type === ValueErrorType.StringPattern) return `Please enter a valid ${name}`
  if (type === ValueErrorType.StringMaxLength) {
    return `Please keep the ${name} under ${String(schema.maxLength)} characters`
  }
  return error.message
}

const segments = (pointer: string): string[] => pointer.split('/').filter((segment) => segment !== '')

const adapters = new WeakMap<TSchema, StandardSchemaV1>()

// TypeBox 0.34 predates Standard Schema; unknown fields are dropped and defaults applied before the check
export const fromTypeBox = <T extends TSchema>(schema: T): StandardSchemaV1<Static<T>, Static<T>> => {
  const cached = adapters.get(schema)
  if (cached) return cached
  const adapter: StandardSchemaV1<Static<T>, Static<T>> = {
    '~standard': {
      version: 1,
      vendor: 'typebox',
      validate: (input) => {
        const value: unknown = Value.Default(schema, Value.Clean(schema, structuredClone(input)))
        if (Value.Check(schema, value)) return { value }
        return {
          issues: [...Value.Errors(schema, value)].map((error) => ({
            message: messageFor(error),
            path: segments(error.path),
          })),
        }
      },
    },
  }
  adapters.set(schema, adapter)
  return adapter
}
