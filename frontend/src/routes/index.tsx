import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { AppShell } from '#/components/layout/app-shell'
import { FieldError } from '#/components/layout/failure-banner'
import { Button } from '#/components/ui/button'
import { inputVariants, invalidProps } from '#/components/ui/field'
import { FindTenantInput } from '#/forms/schemas'
import { validateForm } from '#/forms/validate-form'
import { cn } from '#/lib/cn'
import { findTenantByEmail, rememberedTenant } from '#/server/functions/discovery'

type Search = { error?: string; next?: string }

const Discover = () => {
  const navigate = useNavigate()
  const { error, next } = Route.useSearch()
  const [message, setMessage] = useState<string | null>(null)
  const [fields, setFields] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  // The same schema path as every other form: structure here, meaning (does the domain belong to a tenant) there.
  const find = async (form: FormData) => {
    setMessage(null)
    const checked = validateForm(FindTenantInput, form)
    setFields(checked.fields)
    if (!checked.value) return
    setBusy(true)
    const res = await findTenantByEmail({ data: checked.value })
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
              className={cn(inputVariants({ invalid: Boolean(fields.email) }), 'px-3 py-2')}
              placeholder="you@yourbank.example"
              autoComplete="email"
              {...invalidProps('email', fields.email)}
            />
          </label>
          <FieldError id="email-error" message={fields.email} />
          <Button type="submit" disabled={busy}>
            Continue
          </Button>
        </form>
        {message && <output className="block text-sm text-neutral-700">{message}</output>}
      </div>
    </AppShell>
  )
}

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
