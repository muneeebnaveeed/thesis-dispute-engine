import { serverEnv } from '#/server/runtime/env'

// the nonce is what lets Start's own inline scripts run under a strict CSP
export const securityHeaders = (headers: Headers, nonce: string) => {
  const env = serverEnv()
  const keycloakOrigin = new URL(env.publicKeycloakUrl).origin
  const csp = [
    "default-src 'self'",
    `script-src 'self' 'nonce-${nonce}'`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data:",
    "font-src 'self'",
    "connect-src 'self'",
    `form-action 'self' ${keycloakOrigin}`,
    "frame-ancestors 'none'",
    "base-uri 'self'",
    "object-src 'none'",
  ]
  headers.set('Content-Security-Policy', csp.join('; '))
  headers.set('X-Content-Type-Options', 'nosniff')
  headers.set('Referrer-Policy', 'strict-origin-when-cross-origin')
  headers.set('X-Frame-Options', 'DENY')
  headers.set('Permissions-Policy', 'camera=(), microphone=(), geolocation=()')
  headers.set('Cross-Origin-Opener-Policy', 'same-origin')
  if (env.secureCookies) headers.set('Strict-Transport-Security', 'max-age=31536000; includeSubDomains')
}
