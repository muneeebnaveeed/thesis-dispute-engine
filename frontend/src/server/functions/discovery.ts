import { createServerFn } from '@tanstack/react-start'
import { getCookie } from '@tanstack/react-start/server'
import { Type } from '@sinclair/typebox'

import { FindTenantInput } from '#/forms/schemas'
import { rememberedTenantCookie } from '#/server/auth/session-impl'
import { parse } from '#/server/runtime/fn'
import { tenantByEmail, tenantBySlug } from '#/server/runtime/tenants'

const TenantSlug = Type.String({ pattern: '^[a-z0-9-]{1,63}$' })

export const rememberedTenant = createServerFn({ method: 'GET' }).handler(
  async (): Promise<string | null> => {
    const rememberedSlug = getCookie(rememberedTenantCookie)
    if (!rememberedSlug) return null
    return (await tenantBySlug(rememberedSlug)) ? rememberedSlug : null
  },
)

export const findTenantByEmail = createServerFn({ method: 'POST' })
  .validator(parse(FindTenantInput))
  .handler(async ({ data: { email } }): Promise<{ slug: string } | { error: string }> => {
    const tenant = await tenantByEmail(email)
    return tenant
      ? { slug: tenant.slug }
      : {
          error:
            'We could not find an organisation for that email address. Ask your administrator for your sign-in link.',
        }
  })

export const tenantExists = createServerFn({ method: 'GET' })
  .validator(parse(TenantSlug))
  .handler(async ({ data: slug }): Promise<{ slug: string; name: string } | null> => {
    const tenant = await tenantBySlug(slug)
    return tenant ? { slug: tenant.slug, name: tenant.name } : null
  })
