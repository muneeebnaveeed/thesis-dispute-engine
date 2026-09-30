import { QueryCache, QueryClient } from '@tanstack/react-query'
import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { getGlobalStartContext } from '@tanstack/react-start'

import { staleBuildGuard } from './lib/stale-build'
import { routeTree } from './routeTree.gen'

export const getRouter = () => {
  // one client per router: SSR requests must not share a cache
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 30_000, retry: false, refetchOnWindowFocus: false } },
    // a read that throws failed as a call (problems arrive as data), which is how a stale build shows in the browser
    queryCache: new QueryCache({
      onError: () => {
        if (typeof window !== 'undefined') void staleBuildGuard().check()
      },
    }),
  })
  // the strict CSP in security-headers.ts depends on this nonce
  const startContext = getGlobalStartContext() as { nonce?: string } | undefined
  const router = createTanStackRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    ...(startContext?.nonce ? { ssr: { nonce: startContext.nonce } } : {}),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
