import { afterEach, vi } from 'vitest'

vi.mock('#/server/auth/session', () => ({
  getAccessToken: vi.fn<() => Promise<{ token: string; expiresAt: string } | null>>(),
}))

import { getAccessToken } from '#/server/auth/session'
import { createBrowserApi, forgetToken } from './browser'

const later = () => new Date(Date.now() + 300_000).toISOString()

afterEach(() => {
  forgetToken()
  vi.unstubAllGlobals()
  vi.mocked(getAccessToken).mockReset()
})

test('fetches a token once and reuses it while it is fresh', async () => {
  vi.mocked(getAccessToken).mockResolvedValue({ token: 't1', expiresAt: later() })
  const seen: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn<(r: Request) => Promise<Response>>((r) => {
      seen.push(r.headers.get('authorization') ?? '')
      return Promise.resolve(
        new Response('{"status":"ok"}', { status: 200, headers: { 'Content-Type': 'application/json' } }),
      )
    }),
  )
  const api = createBrowserApi('http://api')
  await api.GET('/healthz')
  await api.GET('/healthz')
  expect(seen).toEqual(['Bearer t1', 'Bearer t1'])
  expect(getAccessToken).toHaveBeenCalledTimes(1)
})

test('on 401 asks the server for a new token and retries once', async () => {
  vi.mocked(getAccessToken)
    .mockResolvedValueOnce({ token: 'old', expiresAt: later() })
    .mockResolvedValueOnce({ token: 'new', expiresAt: later() })
  const seen: string[] = []
  vi.stubGlobal(
    'fetch',
    vi.fn<(r: Request) => Promise<Response>>((r) => {
      const auth = r.headers.get('authorization') ?? ''
      seen.push(auth)
      const ok = auth === 'Bearer new'
      return Promise.resolve(
        new Response(ok ? '{"status":"ok"}' : '{}', {
          status: ok ? 200 : 401,
          headers: { 'Content-Type': ok ? 'application/json' : 'application/problem+json' },
        }),
      )
    }),
  )
  const { response } = await createBrowserApi('http://api').GET('/healthz')
  expect(response.status).toBe(200)
  expect(seen).toEqual(['Bearer old', 'Bearer new'])
})

test('a 401 that survives the retry is returned, not looped', async () => {
  vi.mocked(getAccessToken).mockResolvedValue({ token: 'dead', expiresAt: later() })
  const f = vi.fn<(r: Request) => Promise<Response>>(() =>
    Promise.resolve(
      new Response('{}', { status: 401, headers: { 'Content-Type': 'application/problem+json' } }),
    ),
  )
  vi.stubGlobal('fetch', f)
  const { response } = await createBrowserApi('http://api').GET('/healthz')
  expect(response.status).toBe(401)
  expect(f).toHaveBeenCalledTimes(2)
})
