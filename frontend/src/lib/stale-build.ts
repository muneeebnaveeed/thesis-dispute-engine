// Stamped at build time (VITE_BUILD_VERSION, set by the image build) into both the client bundle and the server.
// Unstamped builds are development ones and never consider themselves stale.
export const BUILD_VERSION: string = import.meta.env.VITE_BUILD_VERSION || 'dev'

const CHECK_EVERY_MS = 60_000

type GuardOptions = {
  version: string
  fetchVersion: () => Promise<string>
  reload: () => void
  storage: Pick<Storage, 'getItem' | 'setItem'>
  now?: () => number
}

// A tab outlives the build that served it: after a deploy its server functions and lazy chunks may no longer exist
// on the server. The guard asks the server which build it runs and reloads the tab when they differ, at most once
// per mismatch so a server that keeps disagreeing cannot trap the tab in a reload loop.
export const createStaleBuildGuard = ({
  version,
  fetchVersion,
  reload,
  storage,
  now = Date.now,
}: GuardOptions) => {
  let lastCheck = -Infinity
  const reloadOnce = (reason: string) => {
    const key = `stale-build:${version}:${reason}`
    if (storage.getItem(key)) return
    storage.setItem(key, '1')
    reload()
  }
  const check = async (): Promise<boolean> => {
    if (version === 'dev') return false
    lastCheck = now()
    let server: string
    try {
      server = (await fetchVersion()).trim()
    } catch {
      return false
    }
    if (!server || server === version) return false
    reloadOnce(`server:${server}`)
    return true
  }
  return {
    check,
    onPreloadError: () => {
      if (version !== 'dev') reloadOnce('chunk')
    },
    onVisible: async () => {
      if (now() - lastCheck >= CHECK_EVERY_MS) await check()
    },
  }
}

let installed: ReturnType<typeof createStaleBuildGuard> | undefined

// the browser's guard; server-side rendering never reaches it
export const staleBuildGuard = () =>
  (installed ??= createStaleBuildGuard({
    version: BUILD_VERSION,
    fetchVersion: async () => {
      const res = await fetch('/build-version', { cache: 'no-store' })
      if (!res.ok) throw new Error(`build-version ${res.status}`)
      return res.text()
    },
    reload: () => window.location.reload(),
    storage: window.sessionStorage,
  }))

// wires the guard to the two moments staleness shows without a failed call: a lazy chunk that is gone, and a tab
// coming back after long enough for a deploy to have happened
export const installStaleBuildGuards = (): (() => void) => {
  const guard = staleBuildGuard()
  const onPreloadError = (event: Event) => {
    event.preventDefault()
    guard.onPreloadError()
  }
  const onVisibility = () => {
    if (document.visibilityState === 'visible') void guard.onVisible()
  }
  window.addEventListener('vite:preloadError', onPreloadError)
  document.addEventListener('visibilitychange', onVisibility)
  return () => {
    window.removeEventListener('vite:preloadError', onPreloadError)
    document.removeEventListener('visibilitychange', onVisibility)
  }
}
