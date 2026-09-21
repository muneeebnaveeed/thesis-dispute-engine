import { createServerFn } from '@tanstack/react-start'

import { serverEnv } from './env'

/** The few settings the browser needs; nothing secret crosses here. */
export const publicConfig = createServerFn({ method: 'GET' }).handler(() => {
  const env = serverEnv()
  return {
    apiUrl: env.publicApiUrl,
    // Quick links on the sign-in page for local demos; production tenants type or bookmark their slug.
    tenants: (process.env.WEB_TENANT_LINKS ?? 'otp:OTP Bank,erste:Erste Bank')
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean)
      .map((t) => {
        const [slug = '', ...name] = t.split(':')
        return { slug, name: name.join(':') || slug }
      }),
  }
})
