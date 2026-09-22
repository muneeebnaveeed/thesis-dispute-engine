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

type FrontDoorSearch = { error?: string; next?: string }

const FrontDoor = () => {
  const navigate = useNavigate()
  const { error: signInError, next } = Route.useSearch()
  const [notFoundMessage, setNotFoundMessage] = useState<string | null>(null)
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)

  const findTenantFromForm = async (form: FormData) => {
    setNotFoundMessage(null)
    const validated = validateForm(FindTenantInput, form)
    setFieldErrors(validated.fields)
    if (!validated.value) return
    setBusy(true)
    const found = await findTenantByEmail({ data: validated.value })
    setBusy(false)
    if ('error' in found) setNotFoundMessage(found.error)
    else void navigate({ to: '/$tenant', params: { tenant: found.slug }, search: next ? { next } : {} })
  }

  return (
    <AppShell title="Sign in">
      <div className="max-w-md space-y-6">
        {signInError && (
          <p role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900">
            Sign-in did not complete: {signInError}
          </p>
        )}
        <p className="text-sm text-neutral-600">
          Use the sign-in link your organisation gave you, or enter your work email and we will find it.
        </p>
        <form
          className="space-y-3"
          onSubmit={(event) => {
            event.preventDefault()
            void findTenantFromForm(new FormData(event.currentTarget))
          }}
        >
          <label className="block text-sm">
            Work email
            <input
              name="email"
              type="email"
              className={cn(inputVariants({ invalid: Boolean(fieldErrors.email) }), 'px-3 py-2')}
              placeholder="you@yourbank.example"
              autoComplete="email"
              {...invalidProps('email', fieldErrors.email)}
            />
          </label>
          <FieldError id="email-error" message={fieldErrors.email} />
          <Button type="submit" disabled={busy}>
            Continue
          </Button>
        </form>
        {notFoundMessage && <output className="block text-sm text-neutral-700">{notFoundMessage}</output>}
      </div>
    </AppShell>
  )
}

export const Route = createFileRoute('/')({
  validateSearch: (rawSearch: Record<string, unknown>): FrontDoorSearch => ({
    ...(typeof rawSearch.error === 'string' ? { error: rawSearch.error } : {}),
    ...(typeof rawSearch.next === 'string' ? { next: rawSearch.next } : {}),
  }),
  beforeLoad: async ({ context, search }) => {
    if (context.viewer) throw redirect({ to: '/$tenant', params: { tenant: context.viewer.tenantSlug } })
    if (search.error) return
    const rememberedSlug = await rememberedTenant()
    if (rememberedSlug)
      throw redirect({
        to: '/$tenant',
        params: { tenant: rememberedSlug },
        search: search.next ? { next: search.next } : {},
      })
  },
  component: FrontDoor,
})
