import { CreateDisputeRequest } from '#/api/schemas.gen'
import { formValues, validateForm } from './validate-form'

function fd(entries: Record<string, string>) {
  const f = new FormData()
  for (const [k, v] of Object.entries(entries)) f.set(k, v)
  return f
}

test('empty fields are absent, so an optional field with a default validates', () => {
  expect(formValues(fd({ transactionId: ' abc ', actor: '  ' }))).toEqual({ transactionId: 'abc' })
})

test('a bad uuid is reported under its field before any request', () => {
  const r = validateForm(CreateDisputeRequest, fd({ transactionId: 'nope' }))
  expect(r.value).toBeNull()
  expect(r.fields).toEqual({ transactionId: expect.any(String) })
  expect(r.failure?.kind).toBe('validation')
})

test('a good form yields the cleaned value with defaults applied', () => {
  const r = validateForm(CreateDisputeRequest, fd({ transactionId: '00000000-0000-8000-8000-000000000101' }))
  expect(r.value).toEqual({ transactionId: '00000000-0000-8000-8000-000000000101', actor: 'customer' })
})
