import type { QueryClient } from '@tanstack/react-query'
import { HeadContent, Outlet, Scripts, createRootRouteWithContext } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { RouteError } from '#/components/layout/route-error'
import { getViewer } from '#/server/functions/session'
import { publicConfig } from '#/server/functions/public-config'
import appCss from '../styles.css?url'

const RootDocument = ({ children }: { children: ReactNode }) => {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body className="min-h-screen bg-neutral-50 text-neutral-900 antialiased">
        {children}
        <Scripts />
      </body>
    </html>
  )
}

// Every page knows who is signed in (or that nobody is) and the browser-facing API URL, from one server round trip.
export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  beforeLoad: async () => ({ viewer: await getViewer(), config: await publicConfig() }),
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'Dispute Engine' },
    ],
    links: [{ rel: 'stylesheet', href: appCss }],
  }),
  shellComponent: RootDocument,
  component: Outlet,
  errorComponent: RouteError,
})
