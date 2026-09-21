import {
  classify,
  describe,
  fieldErrors,
  fromProblem,
  isRetryable,
  localValidation,
  retryAfter,
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

test('non-problem failures are classified too', () => {
  expect(classify({ thrown: new TypeError('Failed to fetch') })).toMatchObject({ kind: 'unreachable' })
  expect(classify({ error: '<html>', response: new Response('', { status: 502 }) })).toMatchObject({
    kind: 'unexpected',
    status: 502,
  })
  expect(classify({ error: undefined })).toBeNull()
  expect(describe(localValidation({ label: 'required' })).hint).toMatch(/highlighted/)
})
