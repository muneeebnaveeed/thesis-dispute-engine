import { Outlet, createFileRoute, notFound, redirect } from '@tanstack/react-router'

import { AppShell } from '#/components/layout/app-shell'
import { beginLogin } from '#/server/functions/session'
import { tenantExists } from '#/server/functions/discovery'

type TenantSearch = { next?: string }

export const Route = createFileRoute('/$tenant')({
  validateSearch: (rawSearch: Record<string, unknown>): TenantSearch =>
    typeof rawSearch.next === 'string' ? { next: rawSearch.next } : {},
  beforeLoad: async ({ context, params, location, search }) => {
    const tenant = await tenantExists({ data: params.tenant })
    if (!tenant) throw notFound()
    if (!context.viewer) {
      // ?next comes from the root; the server validates it
      const next = search.next ?? location.pathname
      const { url: realmLoginUrl } = await beginLogin({ data: { slug: params.tenant, next } })
      throw redirect({ href: realmLoginUrl })
    }
    if (search.next) throw redirect({ to: location.pathname, replace: true })
    return { tenant }
  },
  component: Outlet,
  notFoundComponent: () => (
    <AppShell title="Unknown organisation">
      <p className="text-sm text-neutral-600">
        There is no organisation at this address. Check the link you were given.
      </p>
    </AppShell>
  ),
})
