import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { AppShell } from '#/components/app-shell'
import { findTenantByEmail, rememberedTenant } from '#/server/discovery'

type Search = { error?: string; next?: string }

// The root only decides where to go: a remembered tenant takes over, otherwise a work email finds the tenant.
export const Route = createFileRoute('/')({
  validateSearch: (s: Record<string, unknown>): Search => ({
    ...(typeof s.error === 'string' ? { error: s.error } : {}),
    ...(typeof s.next === 'string' ? { next: s.next } : {}),
  }),
  beforeLoad: async ({ context, search }) => {
    if (context.viewer) throw redirect({ to: '/$tenant', params: { tenant: context.viewer.tenantSlug } })
    if (search.error) return
    const slug = await rememberedTenant()
    if (slug)
      throw redirect({
        to: '/$tenant',
        params: { tenant: slug },
        search: search.next ? { next: search.next } : {},
      })
  },
  component: Discover,
})

const input =
  'w-full rounded-md border border-neutral-300 px-3 py-2 text-sm focus:border-neutral-500 focus:outline-none'
const button =
  'rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white hover:bg-neutral-700 disabled:opacity-50'

function Discover() {
  const navigate = useNavigate()
  const { error, next } = Route.useSearch()
  const [message, setMessage] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  async function find(form: FormData) {
    const raw = form.get('email')
    const email = typeof raw === 'string' ? raw.trim() : ''
    if (!email) return
    setBusy(true)
    setMessage(null)
    const res = await findTenantByEmail({ data: email })
    setBusy(false)
    if ('error' in res) setMessage(res.error)
    else void navigate({ to: '/$tenant', params: { tenant: res.slug }, search: next ? { next } : {} })
  }

  return (
    <AppShell title="Sign in">
      <div className="max-w-md space-y-6">
        {error && (
          <p role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900">
            Sign-in did not complete: {error}
          </p>
        )}
        <p className="text-sm text-neutral-600">
          Use the sign-in link your organisation gave you, or enter your work email and we will find it.
        </p>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault()
            void find(new FormData(e.currentTarget))
          }}
        >
          <label className="block text-sm">
            Work email
            <input
              name="email"
              type="email"
              className={input}
              placeholder="you@yourbank.example"
              autoComplete="email"
            />
          </label>
          <button type="submit" className={button} disabled={busy}>
            Continue
          </button>
        </form>
        {message && <output className="block text-sm text-neutral-700">{message}</output>}
      </div>
    </AppShell>
  )
}
