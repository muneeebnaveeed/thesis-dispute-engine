import type { TSchema } from '@sinclair/typebox'

import { fieldErrors, localValidation, type Failure, type Problem } from '#/api/failure'
import { validate } from '#/api/validate'

// empty strings become absent so optional fields validate as missing
export const formValues = (form: FormData): Record<string, unknown> => {
  const values: Record<string, unknown> = {}
  for (const [name, entry] of form.entries()) {
    if (typeof entry === 'string') {
      const trimmed = entry.trim()
      if (trimmed !== '') values[name] = trimmed
    }
  }
  return values
}

export const validateForm = <T extends TSchema>(schema: T, form: FormData) => {
  const result = validate(schema, formValues(form))
  if (result.ok)
    return { value: result.value, failure: null as Failure | null, fields: {} as Record<string, string> }
  return { value: null, failure: localValidation(result.errors), fields: result.errors }
}

export const serverFieldErrors = (problem: Problem | null | undefined): Record<string, string> =>
  problem ? fieldErrors(problem) : {}
