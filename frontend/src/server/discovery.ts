import { createServerFn } from '@tanstack/react-start'
import { getCookie } from '@tanstack/react-start/server'

import { rememberedTenantCookie } from './auth/session-impl'
import { tenantByEmail, tenantBySlug } from './tenants'

/** Where the root page should send a visitor without them typing anything, if we can tell. */
export const rememberedTenant = createServerFn({ method: 'GET' }).handler(
  async (): Promise<string | null> => {
    const slug = getCookie(rememberedTenantCookie)
    if (!slug) return null
    return (await tenantBySlug(slug)) ? slug : null
  },
)

export const findTenantByEmail = createServerFn({ method: 'POST' })
  .inputValidator((email: string) => email)
  .handler(async ({ data: email }): Promise<{ slug: string } | { error: string }> => {
    const t = await tenantByEmail(email)
    return t
      ? { slug: t.slug }
      : {
          error:
            'We could not find an organisation for that email address. Ask your administrator for your sign-in link.',
        }
  })

export const tenantExists = createServerFn({ method: 'GET' })
  .inputValidator((slug: string) => slug)
  .handler(async ({ data: slug }): Promise<{ slug: string; name: string } | null> => {
    const t = await tenantBySlug(slug)
    return t ? { slug: t.slug, name: t.name } : null
  })
