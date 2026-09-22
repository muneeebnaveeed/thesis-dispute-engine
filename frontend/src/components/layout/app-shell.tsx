import { Link, useRouteContext, useRouter } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { Button, buttonVariants } from '#/components/ui/button'
import { cn } from '#/lib/cn'
import { logout } from '#/server/functions/session'

const navLinkClass = cn(buttonVariants({ variant: 'link', size: 'bare' }), 'mr-3')

export const AppShell = ({ title, children }: { title: string; children: ReactNode }) => {
  const { viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  const signOut = async () => {
    const { url: afterLogoutUrl } = await logout()
    if (afterLogoutUrl.startsWith('/')) await router.navigate({ to: afterLogoutUrl })
    else window.location.assign(afterLogoutUrl)
  }
  return (
    <div className="mx-auto max-w-5xl px-6 py-8">
      <header className="mb-8 flex items-baseline justify-between border-b border-neutral-200 pb-4">
        <Link to="/" className="text-sm font-medium tracking-wide text-neutral-500 uppercase">
          Dispute Engine
        </Link>
        <h1 className="text-2xl font-semibold">{title}</h1>
        {viewer ? (
          <span className="text-sm text-neutral-600">
            {viewer.roles.includes('tenant-admin') && (
              <>
                <Link to="/$tenant/keys" params={{ tenant: viewer.tenantSlug }} className={navLinkClass}>
                  Keys
                </Link>
                <Link to="/$tenant/templates" params={{ tenant: viewer.tenantSlug }} className={navLinkClass}>
                  Templates
                </Link>
              </>
            )}
            {viewer.name} <span className="text-neutral-400">at</span> {viewer.tenantSlug}{' '}
            <Button variant="link" size="bare" onClick={() => void signOut()} className="ml-2">
              Sign out
            </Button>
          </span>
        ) : (
          <span className="text-sm text-neutral-400">Not signed in</span>
        )}
      </header>
      <main>{children}</main>
    </div>
  )
}
