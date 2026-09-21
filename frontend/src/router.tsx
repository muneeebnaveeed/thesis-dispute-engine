import { createRouter as createTanStackRouter } from '@tanstack/react-router'
import { getGlobalStartContext } from '@tanstack/react-start'

import { routeTree } from './routeTree.gen'

export function getRouter() {
  // On the server this runs once per request; the nonce minted by the request middleware tags every inline script
  // Start emits, which is what makes the strict CSP in security-headers.ts possible.
  const ctx = getGlobalStartContext() as { nonce?: string } | undefined
  const router = createTanStackRouter({
    routeTree,
    scrollRestoration: true,
    defaultPreload: 'intent',
    defaultPreloadStaleTime: 0,
    ...(ctx?.nonce ? { ssr: { nonce: ctx.nonce } } : {}),
  })
  return router
}

declare module '@tanstack/react-router' {
  interface Register {
    router: ReturnType<typeof getRouter>
  }
}
