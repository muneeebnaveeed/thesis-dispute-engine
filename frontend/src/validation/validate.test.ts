import { Type } from '@sinclair/typebox'
import type { StandardSchemaV1 } from '@standard-schema/spec'

import { FindTenantInput } from '#/forms/schemas'
import { ApplyEventRequest, CreateDisputeRequest, CreateTenantKeyRequest } from '#/api/schemas.gen'
import { is, validate } from './validate'

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

// stands in for Zod, Valibot or any other Standard Schema library
const evenNumber: StandardSchemaV1<unknown, number> = {
  '~standard': {
    version: 1,
    vendor: 'test',
    validate: (input) =>
      typeof input === 'number' && input % 2 === 0
        ? { value: input }
        : { issues: [{ message: 'Please enter an even number', path: [{ key: 'count' }] }] },
  },
}

test('any Standard Schema validates through the same path', () => {
  expect(validate(evenNumber, 4)).toEqual({ ok: true, value: 4 })
  expect(validate(evenNumber, 3)).toEqual({ ok: false, errors: { count: 'Please enter an even number' } })
  expect(is(evenNumber, 4)).toBe(true)
})

test('refuses an asynchronous schema instead of returning a promise as a value', () => {
  const eventually: StandardSchemaV1 = {
    '~standard': { version: 1, vendor: 'test', validate: (input) => Promise.resolve({ value: input }) },
  }
  expect(() => validate(eventually, 1)).toThrow('asynchronous schemas are not supported')
})

test('nested errors are keyed by dotted field names', () => {
  const Answers = Type.Object({ answers: Type.Object({ q1: Type.String({ minLength: 1 }) }) })
  expect(validate(Answers, { answers: { q1: '' } })).toMatchObject({
    errors: { 'answers.q1': 'Please enter the q1' },
  })
})

test('is narrows to the schema without trusting the caller', () => {
  expect(is(ApplyEventRequest.properties.event, 'CLOSE')).toBe(true)
  expect(is(ApplyEventRequest.properties.event, 'DANCE')).toBe(false)
})
