import { vi } from 'vitest'

import { call } from './call'

const problem = (code: string, extra: Record<string, unknown> = {}) => ({
  error: { type: 'urn:x', title: 't', status: 503, code, retryable: true, requestId: 'r', ...extra },
  response: new Response('', { status: 503 }),
})

test('returns data when the call succeeds', async () => {
  const res = await call(() => Promise.resolve({ data: { ok: true }, response: new Response('') }))
  expect(res).toEqual({ data: { ok: true }, failure: null })
})

test('treats a 2xx without a body as success', async () => {
  const res = await call(() => Promise.resolve({ response: new Response(null, { status: 204 }) }))
  expect(res).toEqual({ data: undefined, failure: null })
})

test('reports an empty body on a response that is not ok', async () => {
  const res = await call(() => Promise.resolve({ response: new Response('', { status: 502 }) }))
  expect(res.failure?.kind).toBe('unexpected')
})

test('classifies thrown network errors', async () => {
  const res = await call(() => Promise.reject(new TypeError('Failed to fetch')))
  expect(res.failure?.kind).toBe('unreachable')
})

test('retries once for a short retryable failure only when the call is idempotent', async () => {
  vi.useFakeTimers()
  const run = vi
    .fn<() => Promise<{ data?: unknown; error?: unknown; response?: Response }>>()
    .mockResolvedValueOnce(problem('unavailable', { retryAfterSeconds: 1 }))
    .mockResolvedValueOnce({ data: 'second', response: new Response('') })
  const p = call(run, { idempotent: true })
  await vi.advanceTimersByTimeAsync(1100)
  expect(await p).toEqual({ data: 'second', failure: null })
  expect(run).toHaveBeenCalledTimes(2)

  const once = vi
    .fn<() => Promise<{ data?: unknown; error?: unknown; response?: Response }>>()
    .mockResolvedValue(problem('unavailable', { retryAfterSeconds: 1 }))
  expect((await call(once)).failure?.kind).toBe('unavailable')
  expect(once).toHaveBeenCalledTimes(1)
  vi.useRealTimers()
})

test('does not wait out a long rate limit', async () => {
  const run = vi
    .fn<() => Promise<{ data?: unknown; error?: unknown; response?: Response }>>()
    .mockResolvedValue(problem('rate-limited', { retryAfterSeconds: 40 }))
  const res = await call(run, { idempotent: true })
  expect(res.failure?.kind).toBe('rate-limited')
  expect(run).toHaveBeenCalledTimes(1)
})
