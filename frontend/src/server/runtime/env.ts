/** Server-only configuration. Nothing here is ever imported by browser code. */
const devDefaults = {
  clientSecret: 'dev-frontend-secret',
  serviceKey: 'dev-service-key',
  sessionSecret: 'ZGV2LXNlc3Npb24tc2VjcmV0LTMyLWJ5dGVzLWxvbmc=',
}

export const serverEnv = () => {
  const appUrl = process.env.APP_URL ?? 'http://localhost:3002'
  const https = appUrl.startsWith('https://')
  const env = {
    apiUrl: process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    // Browser-facing URL of the API, for the client that calls it directly with a handed-out access token.
    publicApiUrl:
      process.env.DISPUTE_PUBLIC_API_URL ?? process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    keycloakUrl: process.env.KEYCLOAK_URL ?? 'http://localhost:8180',
    // What browsers use to reach Keycloak, when it differs from the server-to-server URL.
    publicKeycloakUrl: process.env.KEYCLOAK_PUBLIC_URL ?? process.env.KEYCLOAK_URL ?? 'http://localhost:8180',
    clientId: process.env.OIDC_CLIENT_ID ?? 'frontend',
    clientSecret: process.env.OIDC_CLIENT_SECRET ?? devDefaults.clientSecret,
    appUrl,
    // Shared secret for the API's /internal/* endpoints, and the key that seals session payloads.
    serviceKey: process.env.DISPUTE_SERVICE_KEY ?? devDefaults.serviceKey,
    sessionSecret: process.env.SESSION_SECRET ?? devDefaults.sessionSecret,
    // Absolute session lifetime, and how long a session may sit unused before it is over.
    sessionTtlSeconds: Number(process.env.SESSION_TTL_SECONDS ?? 10 * 60 * 60),
    sessionIdleSeconds: Number(process.env.SESSION_IDLE_SECONDS ?? 30 * 60),
    // Cookies are Secure whenever the app is served over https; production is any https deployment.
    secureCookies: https,
    production: https || process.env.APP_ENV === 'production',
  }
  // Dev defaults are fine on a laptop and a breach in production; refuse to serve rather than hope.
  if (env.production) {
    const bad: string[] = []
    if (env.clientSecret === devDefaults.clientSecret) bad.push('OIDC_CLIENT_SECRET')
    if (env.serviceKey === devDefaults.serviceKey) bad.push('DISPUTE_SERVICE_KEY')
    if (env.sessionSecret === devDefaults.sessionSecret) bad.push('SESSION_SECRET')
    if (bad.length)
      throw new Error(`production configuration still uses development defaults for: ${bad.join(', ')}`)
    if (Buffer.from(env.sessionSecret, 'base64').length !== 32)
      throw new Error('SESSION_SECRET must be 32 bytes, base64 encoded')
  }
  return env
}
