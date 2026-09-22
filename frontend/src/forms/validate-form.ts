import type { TSchema } from '@sinclair/typebox'

import { fieldErrors, localValidation, type Failure, type Problem } from '#/api/failure'
import { validate } from '#/api/validate'

/** Reads a form into a plain object: empty strings become absent so optional fields validate as missing. */
export const formValues = (form: FormData): Record<string, unknown> => {
  const out: Record<string, unknown> = {}
  for (const [k, v] of form.entries()) {
    if (typeof v === 'string') {
      const t = v.trim()
      if (t !== '') out[k] = t
    }
  }
  return out
}

/**
 * Client-side structural validation with the same TypeBox schema the server uses, returning either the cleaned
 * value or a validation failure with messages keyed by field, ready for FieldError components.
 */
export const validateForm = <T extends TSchema>(schema: T, form: FormData) => {
  const r = validate(schema, formValues(form))
  if (r.ok) return { value: r.value, failure: null as Failure | null, fields: {} as Record<string, string> }
  const failure = localValidation(r.errors)
  return { value: null, failure, fields: r.errors }
}

/** Field messages from a server-side validation problem, for the same components. */
export const serverFields = (p: Problem | null | undefined): Record<string, string> => {
  return p ? fieldErrors(p) : {}
}
