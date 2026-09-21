import { useRouteContext } from '@tanstack/react-router'

import { AppShell } from './app-shell'

/** Signed in to one organisation, visiting another's address. Never merge the two; make the person choose. */
export function TenantMismatch({ wanted }: { wanted: string }) {
  const { viewer } = useRouteContext({ from: '__root__' })
  return (
    <AppShell title="Different organisation">
      <p className="text-sm text-neutral-700">
        You are signed in to <span className="font-mono">{viewer?.tenantSlug}</span>, but this address belongs
        to <span className="font-mono">{wanted}</span>. Sign out first to continue there.
      </p>
    </AppShell>
  )
}
