import { createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import type { Problem } from '#/api/problem'
import { AppShell } from '#/components/app-shell'
import { ProblemBanner } from '#/components/problem-banner'
import { TenantMismatch } from '#/components/tenant-mismatch'
import { createDispute } from '#/server/disputes'

export const Route = createFileRoute('/$tenant/')({ component: Workbench })

const input =
  'w-full rounded-md border border-neutral-300 px-3 py-2 font-mono text-sm focus:border-neutral-500 focus:outline-none'
const button =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50'

function Workbench() {
  const { tenant } = Route.useParams()
  const { viewer } = useRouteContext({ from: '__root__' })
  const navigate = useNavigate()
  const [problem, setProblem] = useState<Problem | null>(null)
  const [busy, setBusy] = useState(false)
  if (viewer && viewer.tenantSlug !== tenant) return <TenantMismatch wanted={tenant} />

  async function open(form: FormData) {
    setBusy(true)
    setProblem(null)
    const res = await createDispute({ data: { transactionId: form.get('transactionId'), actor: 'analyst' } })
    setBusy(false)
    if (res.problem) setProblem(res.problem)
    else if (res.value)
      void navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: res.value.id } })
  }

  return (
    <AppShell title="Disputes">
      <div className="grid gap-8 md:grid-cols-2">
        <section>
          <h2 className="mb-3 text-lg font-medium">Open a dispute</h2>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              void open(new FormData(e.currentTarget))
            }}
          >
            <label className="block text-sm">
              Transaction ID
              <input
                name="transactionId"
                className={input}
                placeholder="00000000-0000-8000-8000-000000000101"
              />
            </label>
            <button type="submit" className={button} disabled={busy}>
              Open
            </button>
          </form>
          {problem && (
            <div className="mt-4">
              <ProblemBanner problem={problem} />
            </div>
          )}
        </section>
        <section>
          <h2 className="mb-3 text-lg font-medium">Find a dispute</h2>
          <form
            className="space-y-3"
            onSubmit={(e) => {
              e.preventDefault()
              const raw = new FormData(e.currentTarget).get('disputeId')
              const id = typeof raw === 'string' ? raw.trim() : ''
              if (id) void navigate({ to: '/$tenant/disputes/$disputeId', params: { tenant, disputeId: id } })
            }}
          >
            <label className="block text-sm">
              Dispute ID
              <input name="disputeId" className={input} />
            </label>
            <button type="submit" className={button}>
              Show
            </button>
          </form>
        </section>
      </div>
    </AppShell>
  )
}
