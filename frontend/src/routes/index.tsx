import { createFileRoute } from '@tanstack/react-router'

import { AppShell } from '#/components/app-shell'

export const Route = createFileRoute('/')({ component: Home })

function Home() {
  return (
    <AppShell title="Disputes">
      <p className="text-neutral-600">Sign in to see your disputes.</p>
    </AppShell>
  )
}
