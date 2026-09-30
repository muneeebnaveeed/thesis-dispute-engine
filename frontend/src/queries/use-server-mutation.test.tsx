import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import type { Problem } from '#/api/failure'
import type { ApiResult } from '#/api/views'

import { useServerMutation } from './use-server-mutation'

const check = vi.fn<() => Promise<boolean>>(async () => false)
vi.mock('#/lib/stale-build', () => ({ staleBuildGuard: () => ({ check }) }))
vi.mock('@tanstack/react-router', () => ({ isRedirect: () => false, useRouter: () => ({}) }))

const wrapper = ({ children }: { children: ReactNode }) => (
  <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>
)

beforeEach(() => check.mockClear())

test('a call that fails outright asks whether the build went stale', async () => {
  const { result } = renderHook(() => useServerMutation(async () => Promise.reject(new Error('HTTPError'))), {
    wrapper,
  })
  await act(async () => {
    await result.current.mutateAsync(undefined).catch(() => undefined)
  })
  expect(check).toHaveBeenCalledTimes(1)
  expect(result.current.failure?.kind).toBe('unreachable')
})

test('a refusal from the API is an answer, not a stale build', async () => {
  const problem: Problem = {
    type: 'about:blank',
    title: 't',
    status: 422,
    code: 'invalid-fields',
    retryable: false,
    requestId: 'r',
  }
  const refused: ApiResult<never> = { error: problem, response: new Response(null, { status: 422 }) }
  const { result } = renderHook(() => useServerMutation(async () => refused), { wrapper })
  await act(async () => {
    await result.current.mutateAsync(undefined).catch(() => undefined)
  })
  expect(check).not.toHaveBeenCalled()
})
