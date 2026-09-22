const devDefaults = {
  clientSecret: 'dev-frontend-secret',
  serviceKey: 'dev-service-key',
  sessionSecret: 'ZGV2LXNlc3Npb24tc2VjcmV0LTMyLWJ5dGVzLWxvbmc=',
}

export const serverEnv = () => {
  const appUrl = process.env.APP_URL ?? 'http://localhost:3002'
  const servedOverHttps = appUrl.startsWith('https://')
  const env = {
    apiUrl: process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    keycloakUrl: process.env.KEYCLOAK_URL ?? 'http://localhost:8180',
    // browsers may reach Keycloak by a different URL than the server does
    publicKeycloakUrl: process.env.KEYCLOAK_PUBLIC_URL ?? process.env.KEYCLOAK_URL ?? 'http://localhost:8180',
    clientId: process.env.OIDC_CLIENT_ID ?? 'frontend',
    clientSecret: process.env.OIDC_CLIENT_SECRET ?? devDefaults.clientSecret,
    appUrl,
    serviceKey: process.env.DISPUTE_SERVICE_KEY ?? devDefaults.serviceKey,
    sessionSecret: process.env.SESSION_SECRET ?? devDefaults.sessionSecret,
    sessionTtlSeconds: Number(process.env.SESSION_TTL_SECONDS ?? 10 * 60 * 60),
    sessionIdleSeconds: Number(process.env.SESSION_IDLE_SECONDS ?? 30 * 60),
    secureCookies: servedOverHttps,
    production: servedOverHttps || process.env.APP_ENV === 'production',
  }
  // dev defaults in production would be a breach; refuse to serve
  if (env.production) {
    const stillDefault: string[] = []
    if (env.clientSecret === devDefaults.clientSecret) stillDefault.push('OIDC_CLIENT_SECRET')
    if (env.serviceKey === devDefaults.serviceKey) stillDefault.push('DISPUTE_SERVICE_KEY')
    if (env.sessionSecret === devDefaults.sessionSecret) stillDefault.push('SESSION_SECRET')
    if (stillDefault.length)
      throw new Error(
        `production configuration still uses development defaults for: ${stillDefault.join(', ')}`,
      )
    if (Buffer.from(env.sessionSecret, 'base64').length !== 32)
      throw new Error('SESSION_SECRET must be 32 bytes, base64 encoded')
  }
  return env
}
