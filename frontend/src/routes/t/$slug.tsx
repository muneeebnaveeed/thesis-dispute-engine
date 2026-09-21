import { createFileRoute, redirect } from '@tanstack/react-router'

import { beginLogin } from '#/server/auth/session'

type Search = { next?: string }

// /t/<slug> is the tenant's front door: it starts the code flow against that tenant's realm and leaves.
export const Route = createFileRoute('/t/$slug')({
  validateSearch: (s: Record<string, unknown>): Search =>
    typeof s.next === 'string' ? { next: s.next } : {},
  beforeLoad: async ({ params, search }) => {
    const { url } = await beginLogin({
      data: { slug: params.slug, ...(search.next ? { next: search.next } : {}) },
    })
    throw redirect({ href: url })
  },
  component: () => null,
})
