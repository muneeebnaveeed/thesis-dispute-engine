import { serverEnv } from '#/server/runtime/env'

/** Browser hardening for every response. The nonce lets Start's own inline scripts run under a strict CSP. */
export const securityHeaders = (h: Headers, nonce: string) => {
  const env = serverEnv()
  const api = new URL(env.publicApiUrl).origin
  const keycloak = new URL(env.publicKeycloakUrl).origin
  const csp = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    `connect-src 'self' ${api}`,
    `form-action 'self' ${keycloak}`,
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "object-src 'none'",
  ]
  h.set('Content-Security-Policy', csp.join('; '))
  h.set('X-Content-Type-Options', 'nosniff')
  h.set('Referrer-Policy', 'strict-origin-when-cross-origin')
  h.set('X-Frame-Options', 'DENY')
  h.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()')
  h.set('Cross-Origin-Opener-Policy', 'same-origin')
  if (env.secureCookies) h.set('Strict-Transport-Security', 'max-age=31536000; includeSubDomains')
}
