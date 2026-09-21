import { Link, useRouteContext, useRouter } from '@tanstack/react-router'
import type { ReactNode } from 'react'

import { forgetToken } from '#/api/browser'
import { logout } from '#/server/auth/session'

type Props = { title: string; children: ReactNode }

export function AppShell({ title, children }: Props) {
  const { viewer } = useRouteContext({ from: '__root__' })
  const router = useRouter()
  async function signOut() {
    forgetToken()
    const { url } = await logout()
    if (url.startsWith('/')) await router.navigate({ to: url })
    else window.location.assign(url)
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
              <Link
                to="/$tenant/keys"
                params={{ tenant: viewer.tenantSlug }}
                className="mr-3 underline hover:text-neutral-900"
              >
                Keys
              </Link>
            )}
            {viewer.name} <span className="text-neutral-400">at</span> {viewer.tenantSlug}{' '}
            <button
              type="button"
              onClick={() => void signOut()}
              className="ml-2 underline hover:text-neutral-900"
            >
              Sign out
            </button>
          </span>
        ) : (
          <span className="text-sm text-neutral-400">Not signed in</span>
        )}
      </header>
      <main>{children}</main>
    </div>
  )
}
