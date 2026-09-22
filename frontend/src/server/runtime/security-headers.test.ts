import { securityHeaders } from './security-headers'

test('sets a nonce-based CSP that allows the API and Keycloak and nothing else', () => {
  const headers = new Headers()
  securityHeaders(headers, 'abc123')
  const csp = headers.get('Content-Security-Policy') ?? ''
  expect(csp).toContain("script-src 'self' 'nonce-abc123'")
  expect(csp).not.toContain("script-src 'self' 'unsafe-inline'")
  expect(csp).toContain("connect-src 'self'")
  expect(csp).toContain("form-action 'self' http://localhost:8180")
  expect(csp).toContain("frame-ancestors 'none'")
  expect(headers.get('X-Content-Type-Options')).toBe('nosniff')
  expect(headers.get('Strict-Transport-Security')).toBeNull() // plain http locally
})
