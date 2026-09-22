import { QueryClient } from '@tanstack/react-query'
import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { setupRouterSsrQueryIntegration } from '@tanstack/react-router-ssr-query'
import { getGlobalStartContext } from '@tanstack/react-start'

import { routeTree } from './routeTree.gen'

export const getRouter = () => {
  // One QueryClient per router: on the server that is one per request, so no request sees another's cache. The
  // SSR integration dehydrates what loaders fetched into the HTML and hydrates it in the browser, so first paint
  // needs no client fetch and later navigations hit the cache, then the API directly.
  const queryClient = new QueryClient({
    defaultOptions: { queries: { staleTime: 30_000, retry: false, refetchOnWindowFocus: false } },
  })
  // The nonce minted by the request middleware tags every inline script Start emits, which is what makes the
  // strict CSP in security-headers.ts possible.
  const ctx = getGlobalStartContext() as { nonce?: string } | undefined
  const router = createTanStackRouter({
    routeTree,
    context: { queryClient },
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    ...(ctx?.nonce ? { ssr: { nonce: ctx.nonce } } : {}),
  })
  setupRouterSsrQueryIntegration({ router, queryClient })
  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
