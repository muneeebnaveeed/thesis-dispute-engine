import { Outlet, createFileRoute, notFound, redirect } from '@tanstack/react-router'

import { AppShell } from '#/components/layout/app-shell'
import { beginLogin } from '#/server/functions/session'
import { tenantExists } from '#/server/functions/discovery'

// Everything under /<tenant> belongs to one customer. No session: straight to that tenant's realm (silent when the
// realm's SSO cookie is still alive). A session for a different tenant: say so rather than mixing them.
type Search = { next?: string }

export const Route = createFileRoute('/$tenant')({
  validateSearch: (s: Record<string, unknown>): Search =>
    typeof s.next === 'string' ? { next: s.next } : {},
  beforeLoad: async ({ context, params, location, search }) => {
    const tenant = await tenantExists({ data: params.tenant })
    if (!tenant) throw notFound()
    if (!context.viewer) {
      // Come back to the page asked for, or to an explicit ?next handed over by the root; the server validates it.
      const next = search.next ?? location.pathname
      const { url } = await beginLogin({ data: { slug: params.tenant, next } })
      throw redirect({ href: url })
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
