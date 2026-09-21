import { afterEach, vi } from 'vitest'

import { createApi } from '#/api/client'
import { applyDisputeEvent, openDispute } from './disputes-core'

// Exercise the logic behind the server functions against a fake fetch, so the whole path from validation through
// the generated client to the problem mapping is covered without a running API or the Start runtime.
const problem = {
  type: 'urn:dispute-engine:error:invalid-transition',
  title: 'The request conflicts with the current state',
  status: 409,
  code: 'invalid-transition',
  retryable: false,
  requestId: 'r-9',
  allowedEvents: ['CLOSE'],
}
const dispute = {
  id: '01a0c4fd-c9cf-73fb-8ced-2108fc050b3a',
  regime: 'EU_PSD2_CARD',
  state: 'INITIATED',
  appeals: 0,
  version: 1,
  transactionId: '00000000-0000-8000-8000-000000000101',
  accountId: '00000000-0000-8000-8000-000000000001',
  disputedAmount: '125.4000',
  currency: 'EUR',
  openedAt: '2026-09-21T17:22:41Z',
  updatedAt: '2026-09-21T17:22:41Z',
  allowedEvents: ['OPEN_INVESTIGATION'],
  events: [
    {
      seq: 1,
      event: 'OPENED',
      fromState: '',
      toState: 'INITIATED',
      actor: 'me',
      payload: { a: 1 },
      occurredAt: '2026-09-21T17:22:41Z',
    },
  ],
}

const calls: Request[] = []
function fakeFetch(status: number, body: unknown, contentType = 'application/json') {
  return vi.fn<(req: Request) => Promise<Response>>((req) => {
    calls.push(req)
    return Promise.resolve(
      new Response(JSON.stringify(body), { status, headers: { 'Content-Type': contentType } }),
    )
  })
}
afterEach(() => {
  calls.length = 0
  vi.unstubAllGlobals()
})

test('a valid apply sends the tenant key and returns the view', async () => {
  vi.stubGlobal('fetch', fakeFetch(200, dispute))
  const res = await applyDisputeEvent(createApi('http://api', 'tk_test'), dispute.id, {
    event: 'OPEN_INVESTIGATION',
  })
  expect(res.problem).toBeNull()
  expect(res.value?.events[0]?.payload).toEqual({ a: 1 })
  expect(calls[0]?.headers.get('authorization')).toMatch(/^Bearer tk_/)
  expect(calls[0]?.headers.get('idempotency-key')).toBeTruthy()
})

test('a problem response comes back typed, with allowedEvents', async () => {
  vi.stubGlobal('fetch', fakeFetch(409, problem, 'application/problem+json'))
  const res = await applyDisputeEvent(createApi('http://api', 'tk_test'), dispute.id, { event: 'CLOSE' })
  expect(res.value).toBeNull()
  expect(res.problem?.code).toBe('invalid-transition')
  expect(res.problem?.allowedEvents).toEqual(['CLOSE'])
})

test('a structurally invalid body never reaches the API', async () => {
  const f = fakeFetch(200, dispute)
  vi.stubGlobal('fetch', f)
  const res = await openDispute(createApi('http://api', 'tk_test'), { transactionId: 'nope' })
  expect(f).not.toHaveBeenCalled()
  expect(res.problem?.code).toBe('contract-violation')
  expect(res.problem?.errors?.[0]?.field).toBe('/transactionId')
})
