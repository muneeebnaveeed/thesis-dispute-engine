import { Link, useRouteContext } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { AppSidebar } from '#/components/layout/app-sidebar'
import { SidebarInset, SidebarProvider, SidebarTrigger } from '#/components/shadcn/sidebar'
import { TooltipProvider } from '#/components/shadcn/tooltip'

// Signed out there is no tenant and nothing to navigate to, so the page keeps the plain centred layout.
const PlainShell = ({ title, children }: { title: string; children: ReactNode }) => (
  <div>
    <header className="flex items-center justify-between bg-primary px-6 py-3 text-primary-foreground">
      <Link to="/" className="text-sm font-medium tracking-wide uppercase">
        Dispute Engine
      </Link>
      <span className="text-xs opacity-90">Not signed in</span>
    </header>
    <div className="mx-auto max-w-5xl px-6 py-6">
      <h1 className="mb-4 border-b border-border pb-2 text-2xl font-light">{title}</h1>
      <main>{children}</main>
    </div>
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
        <SidebarInset className="min-w-0 bg-background">
          <header className="flex h-12 shrink-0 items-center gap-3 bg-primary px-4 text-primary-foreground">
            <SidebarTrigger className="text-primary-foreground hover:bg-white/15 hover:text-primary-foreground" />
          </header>
          <div className="flex flex-wrap items-baseline justify-between gap-2 px-6 pt-5 pb-4">
            <h1 className="text-2xl font-light">{title}</h1>
            <nav aria-label="Breadcrumb" className="text-[13px] text-muted-foreground">
              <span className="capitalize">{viewer.tenantSlug}</span>
              <span className="px-1.5">/</span>
              <span className="text-foreground">{title}</span>
            </nav>
          </div>
          <main className="min-w-0 px-6 pb-8">{children}</main>
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  )
}
