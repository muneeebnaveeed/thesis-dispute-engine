/** Server-only configuration. Nothing here is ever imported by browser code. */
export function serverEnv() {
  return {
    apiUrl: process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    // Browser-facing URL of the API, for the client that calls it directly with a handed-out access token.
    publicApiUrl:
      process.env.DISPUTE_PUBLIC_API_URL ?? process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    keycloakUrl: process.env.KEYCLOAK_URL ?? 'http://localhost:8180',
    clientId: process.env.OIDC_CLIENT_ID ?? 'frontend',
    clientSecret: process.env.OIDC_CLIENT_SECRET ?? 'dev-frontend-secret',
    appUrl: process.env.APP_URL ?? 'http://localhost:3002',
    // Shared secret for the API's /internal/sessions endpoints, and the key that seals session payloads.
    serviceKey: process.env.DISPUTE_SERVICE_KEY ?? 'dev-service-key',
    sessionSecret: process.env.SESSION_SECRET ?? 'ZGV2LXNlc3Npb24tc2VjcmV0LTMyLWJ5dGVzLWxvbmc=',
    // Absolute session lifetime; Keycloak's own SSO idle timeout ends it sooner when the refresh token dies.
    sessionTtlSeconds: Number(process.env.SESSION_TTL_SECONDS ?? 10 * 60 * 60),
    secureCookies: process.env.NODE_ENV === 'production',
  }
}
