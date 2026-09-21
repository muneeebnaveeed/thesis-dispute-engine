import { ApplyEventRequest, CreateDisputeRequest } from './schemas.gen'
import { validate } from './validate'

test('accepts a well-formed create request and applies defaults', () => {
  expect(validate(CreateDisputeRequest, { transactionId: '00000000-0000-8000-8000-000000000101' })).toEqual({
    ok: true,
    value: { transactionId: '00000000-0000-8000-8000-000000000101', actor: 'customer' },
  })
})

test('reports a bad uuid under its field', () => {
  expect(validate(CreateDisputeRequest, { transactionId: 'nope' })).toMatchObject({
    ok: false,
    errors: { transactionId: expect.any(String) },
  })
})

test('rejects an event outside the enum', () => {
  expect(validate(ApplyEventRequest, { event: 'DANCE' })).toMatchObject({
    ok: false,
    errors: { event: expect.any(String) },
  })
})

test('drops unknown fields instead of failing on them', () => {
  expect(validate(ApplyEventRequest, { event: 'CLOSE', extra: 1 })).toEqual({
    ok: true,
    value: { event: 'CLOSE', actor: 'system' },
  })
})
