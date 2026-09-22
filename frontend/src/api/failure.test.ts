import {
  describe,
  fieldErrors,
  fromProblem,
  isRetryable,
  retryAfter,
  unreachable,
  type Problem,
} from './failure'

const base: Problem = {
  type: 'urn:x',
  title: 't',
  status: 400,
  code: 'contract-violation',
  retryable: false,
  requestId: 'r-1',
}

test('field pointers from the server map to plain field names', () => {
  expect(
    fieldErrors({
      ...base,
      errors: [
        { field: '/transactionId', message: 'a' },
        { field: 'query.cursor', message: 'b' },
        { field: '/payload/note', message: 'c' },
      ],
    }),
  ).toEqual({ transactionId: 'a', cursor: 'b', 'payload.note': 'c' })
})

test('every contract code lands in a kind with the right retry semantics', () => {
  const codes: Problem['code'][] = [
    'contract-violation',
    'malformed-request',
    'invalid-transition',
    'appeals-exhausted',
    'concurrent-update',
    'idempotency-key-reuse',
    'not-found',
    'unauthenticated',
    'forbidden',
    'rate-limited',
    'unavailable',
    'no-regime',
    'unknown-regime',
    'internal',
  ]
  const kinds = codes.map((code) => fromProblem({ ...base, code }).kind)
  expect(kinds).toEqual([
    'validation',
    'validation',
    'conflict',
    'conflict',
    'conflict',
    'conflict',
    'not-found',
    'signed-out',
    'forbidden',
    'rate-limited',
    'unavailable',
    'validation',
    'validation',
    'internal',
  ])
  expect(retryAfter(fromProblem({ ...base, code: 'rate-limited', retryAfterSeconds: 14 }))).toBe(14)
  expect(retryAfter(fromProblem({ ...base, code: 'unavailable' }))).toBe(5)
  expect(isRetryable(fromProblem({ ...base, code: 'concurrent-update' }))).toBe(false)
})

test('a throw from the call itself is the service being unreachable', () => {
  expect(unreachable(new TypeError('Failed to fetch'))).toEqual({
    kind: 'unreachable',
    message: 'Failed to fetch',
  })
  expect(describe(unreachable('x')).title).toMatch(/Cannot reach/)
})

test('ledger refusals point at the fact in the payload', () => {
  const failure = fromProblem({
    ...base,
    status: 422,
    code: 'invalid-liability',
    title: 'the customer liability is not allowed under this regime',
    errors: [{ field: 'body.payload.liability', message: 'exceeds the 50.00 EUR cap under EU_PSD2_CARD' }],
  })
  const fields = failure.kind === 'validation' ? failure.fields : {}
  expect(fields.liability).toMatch(/cap/)
})

test('a core decline is its own kind, with the core answer in the title', () => {
  const failure = fromProblem({
    ...base,
    status: 422,
    code: 'core-declined',
    title: 'the banking core declined the posting',
    detail: 'the banking core declined the posting: exceeds amount limit (61)',
  })
  expect(failure.kind).toBe('declined')
  expect(describe(failure).title).toMatch(/61/)
  expect(isRetryable(failure)).toBe(false)
})
