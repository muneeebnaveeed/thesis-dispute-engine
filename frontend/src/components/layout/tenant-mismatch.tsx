import { useRouteContext } from '@tanstack/react-router'

import { AppShell } from '#/components/layout/app-shell'

export const TenantMismatch = ({ wanted: requestedTenantSlug }: { wanted: string }) => {
  const { viewer } = useRouteContext({ from: '__root__' })
  return (
    <AppShell title="Different organisation">
      <p className="text-sm text-neutral-700">
        You are signed in to <span className="font-mono">{viewer?.tenantSlug}</span>, but this address belongs
        to <span className="font-mono">{requestedTenantSlug}</span>. Sign out first to continue there.
      </p>
    </AppShell>
  )
}
