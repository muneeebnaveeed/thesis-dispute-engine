import { HeadContent, Outlet, Scripts, createRootRoute } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { getViewer } from '#/server/auth/session'
import { publicConfig } from '#/server/public-config'
import appCss from '../styles.css?url'

// Every page knows who is signed in (or that nobody is) and the browser-facing API URL, from one server round trip.
export const Route = createRootRoute({
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
})

function RootDocument({ children }: { children: ReactNode }) {
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
