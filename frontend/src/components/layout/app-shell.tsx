import { Link, useRouteContext } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { AppSidebar } from '#/components/layout/app-sidebar'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '#/components/shadcn/sidebar'
import { TooltipProvider } from '#/components/shadcn/tooltip'

// Signed out there is no tenant and nothing to navigate to, so the page keeps the plain centred layout.
const PlainShell = ({ title, children }: { title: string; children: ReactNode }) => (
  <div className="mx-auto max-w-5xl px-6 py-8">
    <header className="mb-8 flex items-baseline justify-between border-b pb-4">
      <Link to="/" className="text-sm font-medium tracking-wide text-muted-foreground uppercase">
        Dispute Engine
      </Link>
      <h1 className="text-2xl font-semibold">{title}</h1>
      <span className="text-sm text-muted-foreground">Not signed in</span>
    </header>
    <main>{children}</main>
  </div>
)

export const AppShell = ({ title, children }: { title: string; children: ReactNode }) => {
  const { viewer } = useRouteContext({ from: '__root__' })
  if (!viewer) return <PlainShell title={title}>{children}</PlainShell>
  // the sidebar labels its collapsed icons with tooltips, and this version of the registry expects the provider
  // to come from the app
  return (
    <TooltipProvider>
      <SidebarProvider>
        <AppSidebar viewer={viewer} />
        <SidebarInset>
          <header className="flex h-14 items-center gap-3 border-b px-6">
            <SidebarTrigger />
            <h1 className="text-lg font-semibold">{title}</h1>
          </header>
          <main className="min-w-0 px-6 py-8">{children}</main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
