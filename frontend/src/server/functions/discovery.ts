import { createServerFn } from '@tanstack/react-start'
import { getCookie } from '@tanstack/react-start/server'
import { Type } from '@sinclair/typebox'

import { FindTenantInput } from '#/forms/schemas'
import { parse } from '#/server/runtime/fn'

import { rememberedTenantCookie } from '#/server/auth/session-impl'
import { tenantByEmail, tenantBySlug } from '#/server/runtime/tenants'

/** Where the root page should send a visitor without them typing anything, if we can tell. */
export const rememberedTenant = createServerFn({ method: 'GET' }).handler(
  async (): Promise<string | null> => {
    const slug = getCookie(rememberedTenantCookie)
    if (!slug) return null
    return (await tenantBySlug(slug)) ? slug : null
  },
)

export const findTenantByEmail = createServerFn({ method: 'POST' })
  .validator(parse(FindTenantInput))
  .handler(async ({ data }): Promise<{ slug: string } | { error: string }> => {
    const t = await tenantByEmail(data.email)
    return t
      ? { slug: t.slug }
      : {
          error:
            'We could not find an organisation for that email address. Ask your administrator for your sign-in link.',
        }
  })

export const tenantExists = createServerFn({ method: 'GET' })
  .validator(parse(Type.String({ pattern: '^[a-z0-9-]{1,63}$' })))
  .handler(async ({ data: slug }): Promise<{ slug: string; name: string } | null> => {
    const t = await tenantBySlug(slug)
    return t ? { slug: t.slug, name: t.name } : null
  })
