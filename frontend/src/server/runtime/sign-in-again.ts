import { redirect } from '@tanstack/react-router'
import { getCookie, getRequestHeader, getRequestUrl } from '@tanstack/react-start/server'

import { rememberedTenantCookie } from '#/server/auth/session-impl'

const SLUG = /^[a-z0-9][a-z0-9-]{1,62}$/

// an RPC's own URL is /_serverFn/...; the page it came from is in the Referer (same origin, so always sent)
const pageBeingViewed = (): URL => {
  const requestUrl = getRequestUrl()
  const referer = getRequestHeader('referer')
  if (!referer) return requestUrl
  const refererUrl = new URL(referer, requestUrl)
  return refererUrl.origin === requestUrl.origin ? refererUrl : requestUrl
}

// the session is gone: through the tenant's front door and back to this page, like a fresh visit would
export const signInAgain = () => {
  const page = pageBeingViewed()
  const firstSegment = page.pathname.split('/')[1] ?? ''
  const slug =
    SLUG.test(firstSegment) && firstSegment !== 'auth' ? firstSegment : getCookie(rememberedTenantCookie)
  if (!slug) return redirect({ to: '/' })
  return redirect({ to: '/$tenant', params: { tenant: slug }, search: { next: page.pathname + page.search } })
}
