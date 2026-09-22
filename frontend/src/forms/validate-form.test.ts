import { CreateDisputeRequest } from '#/api/schemas.gen'
import { formValues, validateForm } from './validate-form'

const formOf = (entries: Record<string, string>) => {
  const form = new FormData()
  for (const [name, value] of Object.entries(entries)) form.set(name, value)
  return form
}

test('empty fields are absent, so an optional field with a default validates', () => {
  expect(formValues(formOf({ transactionId: ' abc ', actor: '  ' }))).toEqual({ transactionId: 'abc' })
})

test('a bad uuid is reported under its field before any request', () => {
  const validated = validateForm(CreateDisputeRequest, formOf({ transactionId: 'nope' }))
  expect(validated.value).toBeNull()
  expect(validated.fields).toEqual({ transactionId: expect.any(String) })
  expect(validated.failure?.kind).toBe('validation')
})

test('a good form yields the cleaned value with defaults applied', () => {
  const validated = validateForm(
    CreateDisputeRequest,
    formOf({ transactionId: '00000000-0000-8000-8000-000000000101' }),
  )
  expect(validated.value).toEqual({
    transactionId: '00000000-0000-8000-8000-000000000101',
    actor: 'customer',
  })
})
