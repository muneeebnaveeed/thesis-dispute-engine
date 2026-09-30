import { describe, expect, test, vi } from 'vitest'

import { createStaleBuildGuard } from './stale-build'

const memoryStorage = () => {
  const items = new Map<string, string>()
  return {
    getItem: (key: string) => items.get(key) ?? null,
    setItem: (key: string, value: string) => void items.set(key, value),
  }
}

const guard = (version: string, serverVersion: string | Error, now = () => 0) => {
  const reload = vi.fn<() => void>()
  const fetchVersion = vi.fn<() => Promise<string>>(async () => {
    if (serverVersion instanceof Error) throw serverVersion
    return serverVersion
  })
  const storage = memoryStorage()
  return {
    reload,
    fetchVersion,
    storage,
    g: createStaleBuildGuard({ version, fetchVersion, reload, storage, now }),
  }
}

describe('stale build guard', () => {
  test('the same build on both sides changes nothing', async () => {
    const { g, reload } = guard('v2', 'v2')
    expect(await g.check()).toBe(false)
    expect(reload).not.toHaveBeenCalled()
  })

  test('a newer server reloads the tab, once per server build', async () => {
    const { g, reload, storage } = guard('v1', 'v2')
    expect(await g.check()).toBe(true)
    expect(await g.check()).toBe(true)
    expect(reload).toHaveBeenCalledTimes(1)
    // a later deploy is a new mismatch and may reload again
    const next = createStaleBuildGuard({
      version: 'v1',
      fetchVersion: async () => 'v3',
      reload,
      storage,
      now: () => 0,
    })
    await next.check()
    expect(reload).toHaveBeenCalledTimes(2)
  })

  test('an unreachable server is an outage, not a stale build', async () => {
    const { g, reload } = guard('v1', new Error('offline'))
    expect(await g.check()).toBe(false)
    expect(reload).not.toHaveBeenCalled()
  })

  test('an unstamped build (development) never checks', async () => {
    const { g, reload, fetchVersion } = guard('dev', 'v2')
    expect(await g.check()).toBe(false)
    expect(fetchVersion).not.toHaveBeenCalled()
    g.onPreloadError()
    expect(reload).not.toHaveBeenCalled()
  })

  test('a chunk that failed to load reloads once for this build', () => {
    const { g, reload } = guard('v1', 'v1')
    g.onPreloadError()
    g.onPreloadError()
    expect(reload).toHaveBeenCalledTimes(1)
  })

  test('returning to the tab checks at most once a minute', async () => {
    let t = 0
    const { g, fetchVersion } = guard('v1', 'v1', () => t)
    await g.onVisible()
    await g.onVisible()
    expect(fetchVersion).toHaveBeenCalledTimes(1)
    t = 61_000
    await g.onVisible()
    expect(fetchVersion).toHaveBeenCalledTimes(2)
  })
})
