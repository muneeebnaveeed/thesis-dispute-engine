import { FindTenantInput } from '#/forms/schemas'
import { ApplyEventRequest, CreateDisputeRequest, CreateTenantKeyRequest } from './schemas.gen'
import { validate } from './validate'

test('accepts a well-formed create request and applies defaults', () => {
  expect(validate(CreateDisputeRequest, { transactionId: '00000000-0000-8000-8000-000000000101' })).toEqual({
    ok: true,
    value: { transactionId: '00000000-0000-8000-8000-000000000101', actor: 'customer' },
  })
})

test('messages say what to do, named after the field', () => {
  expect(validate(CreateDisputeRequest, { transactionId: 'nope' })).toMatchObject({
    ok: false,
    errors: { transactionId: 'Please enter a valid transaction ID' },
  })
  expect(validate(CreateDisputeRequest, {})).toMatchObject({
    errors: { transactionId: 'Please enter the transaction ID' },
  })
  expect(validate(ApplyEventRequest, { event: 'DANCE' })).toMatchObject({
    ok: false,
    errors: { event: 'Please select the event' },
  })
  expect(validate(ApplyEventRequest, {})).toMatchObject({ errors: { event: 'Please select the event' } })
  expect(validate(CreateTenantKeyRequest, { label: '' })).toMatchObject({
    errors: { label: 'Please enter the label' },
  })
  expect(validate(CreateTenantKeyRequest, { label: 'x'.repeat(81) })).toMatchObject({
    errors: { label: 'Please keep the label under 80 characters' },
  })
  expect(validate(FindTenantInput, { email: 'nope' })).toMatchObject({
    errors: { email: 'Please enter a valid email address' },
  })
})

test('drops unknown fields instead of failing on them', () => {
  expect(validate(ApplyEventRequest, { event: 'CLOSE', extra: 1 })).toEqual({
    ok: true,
    value: { event: 'CLOSE', actor: 'system' },
  })
})
