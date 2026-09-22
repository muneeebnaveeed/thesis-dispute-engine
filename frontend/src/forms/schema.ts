import type { Static, TSchema } from '@sinclair/typebox'

import { validate } from '#/api/validate'

// empty strings become absent so optional fields validate as missing and defaults apply
const withoutBlanks = (value: unknown): unknown => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return value
  return Object.fromEntries(
    Object.entries(value)
      .map(([name, entry]) => [name, typeof entry === 'string' ? entry.trim() : withoutBlanks(entry)])
      .filter(([, entry]) => entry !== ''),
  )
}

// a TanStack Form validator: structural rules from the contract schema, one message per field
export const schemaValidator =
  (schema: TSchema) =>
  ({ value }: { value: unknown }) => {
    const result = validate(schema, withoutBlanks(value))
    return result.ok ? undefined : { fields: result.errors }
  }

// the cleaned, defaulted value a form submits; only called after schemaValidator has passed
export const parsed = <T extends TSchema>(schema: T, value: unknown): Static<T> => {
  const result = validate(schema, withoutBlanks(value))
  if (!result.ok) throw new Error(`form submitted before validation: ${JSON.stringify(result.errors)}`)
  return result.value
}
