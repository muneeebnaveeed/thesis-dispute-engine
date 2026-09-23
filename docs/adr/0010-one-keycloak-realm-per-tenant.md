# 0010: One Keycloak realm per tenant, tokens verified by the API

**Status:** accepted, 2026-09-21; refines ADR 0009

## Context

ADR 0009 chose Keycloak for analysts and left the realm layout open. The tenants are banks and PSPs;
some will require that their user directory, password and MFA policy, login page and identity
providers be separated from every other customer's at the identity-system level, not only by a
claim. Keycloak offers two shapes: one realm with an organisation per tenant, or a realm per tenant.

## Decision

- One realm per tenant. The realm name is the tenant's `slug`; the tenant row records the realm's
  issuer URL. Realms are rendered from `deploy/keycloak/realm.template.json` by
  `deploy/keycloak/render-realms.sh` for the seeded tenants and imported on Keycloak start; the
  rendered files are committed and `make api:generate-check` fails on drift. Onboarding a tenant means
  a row plus a rendered realm.
- Each realm carries a `dispute-api` client scope that puts `dispute-api` in the audience and a
  hardcoded `tenant_id` claim in every token, plus `roles`, `preferred_username` and `email`.
- The API verifies analyst tokens itself (`auth.OIDC`, `coreos/go-oidc`): it reads the unverified
  issuer to pick the realm, refuses issuers no tenant owns, builds one verifier per issuer
  (discovery plus JWKS, cached, keys refreshed on rotation), checks signature, expiry and audience,
  and refuses a `tenant_id` claim that disagrees with the issuer's tenant. The issuer decides the
  tenant; the claim is a tripwire for a misconfigured realm.
- `auth.Bearer` accepts both credential kinds on the same header: a JWT is an analyst token, anything
  else a tenant key. Both end in the same tenant context; analyst requests also carry a principal.
- Browsers call the API directly with a short-lived access token they obtain from the frontend
  server's session (ADR 0011 covers the session); the API allows the frontend origin through CORS,
  answering preflights before authentication.
- Keycloak runs from the official image in dev mode locally, Postgres-backed in its own database and
  role so state survives restarts, health-checked on the management port.

## Consequences

- Easier: a customer's identity configuration is fully theirs, including future SSO federation and
  delegated realm administration; a compromised or misconfigured realm affects one tenant; the
  isolation story in the write-up is uniform (rows by RLS, credentials by key or realm, sessions by
  realm).
- Harder: N realms to render and import, N issuers to trust (discovered lazily, not configured), a
  tenant must be chosen before login (slug in the URL), cross-tenant operator views need an admin
  outside every realm, and Keycloak start time grows with realm count. Organisations in one realm
  remain the fallback if realm count ever becomes the constraint; the template makes that a
  rendering change, not a data migration.
