# 0009: API keys for tenant systems, Keycloak OIDC for people

**Status:** accepted, 2026-09-21; completes ADR 0008

## Context

Two populations call the engine. Tenant systems (an issuer's core banking or app backend opening a
dispute, a case tool acknowledging a chargeback, a batch replaying events) authenticate as the
tenant, run unattended from configuration, and expect a credential they can store. Analysts in the
frontend authenticate as a person within a tenant and expect a login, sessions, MFA and password
recovery. Hand-rolling the second is where projects lose weeks and still end up with a weakness in
the write-up; hand-rolling the first is sixty lines.

## Decision

- Tenant systems present an API key: `Authorization: Bearer dk_...`, 32 random bytes shown once at
  creation, stored as a SHA-256 hash in `api_keys` with `tenant_id`, `label` and `revoked_at`.
  `cmd/apikey` issues, lists and revokes as the schema owner; the seed installs two fixed dev keys.
  `api_keys` is not under row-level security because the lookup is what establishes the tenant.
- The OpenAPI spec is the policy: a top-level `security: [bearerAuth]`, `security: []` on the
  health endpoints. `auth.Bearer` resolves whatever key is presented and records the outcome; the
  request validator's authentication hook (`auth.Required`) runs only for secured operations and
  turns a missing tenant into a 401 `unauthenticated` problem. Handlers never read headers.
- People sign in through Keycloak (self-hosted in compose, realm configuration in the repo) using
  OIDC. The backend will validate the ID token with `coreos/go-oidc`, map a `tenant_id` claim to the
  same `tenant` context, and add nothing else: everything below the middleware is shared with the
  API-key path. Keycloak's organisations map one-to-one onto tenants.
- Keycloak client credentials could later replace API keys for tenant systems with one middleware
  and one ADR; not now, because keys work without Keycloak running (CI, probe, curl) and are what
  integrators expect.

## Consequences

- Easier: a single tenant context for both populations; auth policy is read from the same contract
  the client SDKs are generated from; a leaked key is grep-able by prefix and revocable without
  touching others.
- Harder: one more container in the frontend phase; a rotation story for keys (issue new, switch,
  revoke old) that the CLI supports but nothing automates; Keycloak realm export must be kept in
  sync with the tenants table.
