import { securityHeaders } from './security-headers'

test('sets a nonce-based CSP that allows the API and Keycloak and nothing else', () => {
  const h = new Headers()
  securityHeaders(h, 'abc123')
  const csp = h.get('Content-Security-Policy') ?? ''
  expect(csp).toContain("script-src 'self' 'nonce-abc123'")
  expect(csp).not.toContain("script-src 'self' 'unsafe-inline'")
  expect(csp).toContain("connect-src 'self' http://localhost:8090")
  expect(csp).toContain("form-action 'self' http://localhost:8180")
  expect(csp).toContain("frame-ancestors 'none'")
  expect(h.get('X-Content-Type-Options')).toBe('nosniff')
  expect(h.get('Strict-Transport-Security')).toBeNull() // plain http locally
})
