import { createFileRoute, redirect, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'

import { AppShell } from '#/components/layout/app-shell'
import { submitting, useAppForm } from '#/forms/app-form'
import { parsed, schemaValidator } from '#/forms/schema'
import { FindTenantInput } from '#/forms/schemas'
import { findTenantByEmail, rememberedTenant } from '#/server/functions/discovery'

type FrontDoorSearch = { error?: string; next?: string }

const FrontDoor = () => {
  const navigate = useNavigate()
  const { error: signInError, next } = Route.useSearch()
  const [notFoundMessage, setNotFoundMessage] = useState<string | null>(null)
  const finder = useAppForm({
    defaultValues: { email: '' },
    validators: { onSubmit: schemaValidator(FindTenantInput) },
    onSubmit: async ({ value }) => {
      setNotFoundMessage(null)
      const found = await findTenantByEmail({ data: parsed(FindTenantInput, value) })
      if ('error' in found) setNotFoundMessage(found.error)
      else await navigate({ to: '/$tenant', params: { tenant: found.slug }, search: next ? { next } : {} })
    },
  })

  return (
    <AppShell title="Sign in">
      <div className="max-w-md space-y-6">
        {signInError && (
          <p role="alert" className="rounded-md border border-red-200 bg-red-50 p-3 text-sm text-red-900">
            Sign-in did not complete: {signInError}
          </p>
        )}
        <p className="text-sm text-muted-foreground">
          Use the sign-in link your organisation gave you, or enter your work email and we will find it.
        </p>
        <form className="space-y-3" onSubmit={submitting(finder)}>
          <finder.AppField name="email">
            {(field) => (
              <field.TextField
                label="Work email"
                type="email"
                inputClassName="px-3 py-2"
                placeholder="you@yourbank.example"
                autoComplete="email"
              />
            )}
          </finder.AppField>
          <finder.AppForm>
            <finder.SubmitButton>Continue</finder.SubmitButton>
          </finder.AppForm>
        </form>
        {notFoundMessage && <output className="block text-sm text-foreground">{notFoundMessage}</output>}
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
