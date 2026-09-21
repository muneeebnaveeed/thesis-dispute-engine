import { createFileRoute, useNavigate, useRouteContext } from '@tanstack/react-router'
import { useState } from 'react'

import type { Problem } from '#/api/problem'
import { AppShell } from '#/components/app-shell'
import { ProblemBanner } from '#/components/problem-banner'
import { createDispute } from '#/server/disputes'

type Search = { error?: string; next?: string }

export const Route = createFileRoute('/')({
  validateSearch: (s: Record<string, unknown>): Search => ({
    ...(typeof s.error === 'string' ? { error: s.error } : {}),
    ...(typeof s.next === 'string' ? { next: s.next } : {}),
  }),
  component: Home,
})

const input =
  'w-full rounded-md border border-neutral-300 px-3 py-2 font-mono text-sm focus:border-neutral-500 focus:outline-none'
const button =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50'

function Home() {
  const { viewer, config } = useRouteContext({ from: '__root__' })
  return (
    <AppShell title={viewer ? 'Disputes' : 'Sign in'}>
      {viewer ? <Workbench /> : <SignIn tenants={config.tenants} />}
    </AppShell>
  )
}

function SignIn({ tenants }: { tenants: { slug: string; name: string }[] }) {
  const navigate = useNavigate()
  const { error, next } = Route.useSearch()
  const after = next ? { next } : {}
  return (
    <div className="max-w-md space-y-6">
      {error && (
        <p role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900">
          Sign-in did not complete: {error}
        </p>
      )}
      <form
        className="space-y-3"
        onSubmit={(e) => {
          e.preventDefault()
          const raw = new FormData(e.currentTarget).get('slug')
          const slug = typeof raw === 'string' ? raw.trim().toLowerCase() : ''
          if (slug) void navigate({ to: '/t/$slug', params: { slug }, search: after })
        }}
      >
        <label className="block text-sm">
          Your organisation
          <input name="slug" className={input} placeholder="otp" autoComplete="organization" />
        </label>
        <button type="submit" className={button}>
          Continue
        </button>
      </form>
      {tenants.length > 0 && (
        <p className="text-sm text-neutral-600">
          Demo tenants:{' '}
          {tenants.map((t, i) => (
            <span key={t.slug}>
              {i > 0 && ', '}
              <a
                href={`/t/${t.slug}${next ? `?next=${encodeURIComponent(next)}` : ''}`}
                className="underline"
              >
                {t.name}
              </a>
            </span>
          ))}
        </p>
      )}
    </div>
  )
}

function Workbench() {
  const navigate = useNavigate()
  const [problem, setProblem] = useState<Problem | null>(null)
  const [busy, setBusy] = useState(false)

  async function open(form: FormData) {
    setBusy(true)
    setProblem(null)
    const res = await createDispute({ data: { transactionId: form.get('transactionId'), actor: 'analyst' } })
    setBusy(false)
    if (res.problem) setProblem(res.problem)
    else if (res.value) void navigate({ to: '/disputes/$disputeId', params: { disputeId: res.value.id } })
  }

  return (
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
            if (id) void navigate({ to: '/disputes/$disputeId', params: { disputeId: id } })
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
  )
}
