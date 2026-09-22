import { createFileRoute } from '@tanstack/react-router'

// imported inside the handler so nothing server-only leaks into this route file's client bundle
export const Route = createFileRoute('/auth/callback')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const { completeLogin } = await import('#/server/auth/session-impl')
        // not Response.redirect(): its headers are immutable and Start must still attach the session cookie
        const seeOther = (to: string) =>
          new Response(null, { status: 303, headers: { Location: new URL(to, request.url).href } })
        try {
          return seeOther(await completeLogin(new URL(request.url)))
        } catch (thrown) {
          const reason = thrown instanceof Error ? thrown.message : 'sign-in failed'
          return seeOther(`/?error=${encodeURIComponent(reason)}`)
        }
      },
    },
  },
})
