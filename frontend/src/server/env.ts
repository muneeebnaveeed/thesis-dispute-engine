/** Server-only configuration. The tenant key stays here until analysts sign in through Keycloak. */
export function serverEnv() {
  return {
    apiUrl: process.env.DISPUTE_API_URL ?? 'http://localhost:8090',
    tenantKey: process.env.DISPUTE_TENANT_KEY ?? 'tk_dev_tenant_a',
  }
}
