import { createFileRoute } from '@tanstack/react-router'

// Keycloak posts a logout_token here when a realm session ends elsewhere (admin action, SSO logout, disabled user).
export const Route = createFileRoute('/auth/backchannel-logout')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        const { handleBackchannelLogout } = await import('#/server/auth/backchannel')
        const form = await request.formData()
        const logoutToken = form.get('logout_token')
        const ended = typeof logoutToken === 'string' && (await handleBackchannelLogout(logoutToken))
        return new Response(null, { status: ended ? 200 : 400, headers: { 'Cache-Control': 'no-store' } })
      },
    },
  },
})
