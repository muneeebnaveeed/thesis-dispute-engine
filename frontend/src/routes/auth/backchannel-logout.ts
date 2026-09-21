import { createFileRoute } from '@tanstack/react-router'

// Keycloak posts a logout_token here when a realm session ends elsewhere (admin action, SSO logout, disabled user).
export const Route = createFileRoute('/auth/backchannel-logout')({
  server: {
    handlers: {
      POST: async ({ request }) => {
        const { handleBackchannelLogout } = await import('#/server/auth/backchannel')
        const form = await request.formData()
        const token = form.get('logout_token')
        const ok = typeof token === 'string' && (await handleBackchannelLogout(token))
        return new Response(null, { status: ok ? 200 : 400, headers: { 'Cache-Control': 'no-store' } })
      },
    },
  },
})
