import { createFileRoute } from '@tanstack/react-router'

// Keycloak redirects here with code and state; the session module turns them into a live session. The import is
// inside the handler so nothing server-only leaks into the client bundle of this route file.
export const Route = createFileRoute('/auth/callback')({
  server: {
    handlers: {
      GET: async ({ request }) => {
        const { completeLogin } = await import('#/server/auth/session-impl')
        // Not Response.redirect(): its headers are immutable and Start must still attach the session cookie.
        const redirect = (to: string) =>
          new Response(null, { status: 303, headers: { Location: new URL(to, request.url).href } })
        try {
          return redirect(await completeLogin(new URL(request.url)))
        } catch (err) {
          const reason = err instanceof Error ? err.message : 'sign-in failed'
          return redirect(`/?error=${encodeURIComponent(reason)}`)
        }
      },
    },
  },
})
