# Authentication

How callers become a tenant, and how analysts sign in. Decisions: ADR 0009 (two credential kinds), 0010 (a
Keycloak realm per tenant, verified by the API), 0011 (server-side sessions, sealed, stored by the API).

## Two populations, one tenant context

| Who | Credential | Issued by | Ends up as |
| --- | --- | --- | --- |
| A tenant's own systems | tenant key `tk_...` (`Authorization: Bearer`) | operator (`cmd/tenantkey`) | `tenant` in the request context |
| A tenant's analysts | access token from the tenant's Keycloak realm | Keycloak, after sign-in | the same `tenant`, plus a principal (subject, roles, realm session id) |

`auth.Bearer` tells them apart by shape, verifies, and everything below sees only the tenant; row-level security does
the isolation. The OpenAPI spec says which operations need which scheme (`bearerAuth`, `serviceKey`); the request
validator enforces it.

## The analyst's path

1. The tenant's front door is `/<slug>` (for example `/otp`). No chooser, nothing to type: an unauthenticated visit
   starts the code flow with PKCE against that tenant's realm and returns to the page asked for.
2. `/` resolves a remembered tenant (cookie set at the last sign-in) or asks for a work email and maps its domain to a
   tenant (`tenants.email_domains`).
3. The callback validates the ID token, mints a fresh session id, seals identity and tokens with `SESSION_SECRET`
   (AES-256-GCM) and stores the blob through `/internal/sessions/{id}`; the browser gets an HttpOnly, SameSite=Lax
   cookie (Secure when served over https).
4. Pages render server-side with the session's access token, and every later action is a server function that
   reads the same token on the server (docs/adr/0020); the browser never sees a token and never calls the API.
   Refresh happens only on the server. When refresh fails the browser is sent back through the front door, which
   is silent while the realm's SSO session lives.
5. Sessions end by sign-out (also ends the Keycloak session), by the idle timeout (`SESSION_IDLE_SECONDS`, 30 min),
   by the absolute TTL (`SESSION_TTL_SECONDS`, 10 h), or by Keycloak's back-channel logout to
   `/auth/backchannel-logout` (admin action, SSO logout, disabled user), which ends every session for that realm
   session id or subject.

## Protections in place

- No token in the browser at all, in storage or in memory; the only browser credential is the session cookie.
- PKCE, `state` and `nonce` on the code flow; the pre-login cookie can never become a live session.
- Sealed session payloads: the API and a database dump hold ciphertext only.
- The token's issuer chooses the realm keys; unknown issuers, wrong audience, expired tokens, tokens signed by another
  realm, and a `tenant_id` claim disagreeing with the issuer are all refused.
- Strict CSP with a per-request nonce, `frame-ancestors 'none'`, nosniff, referrer policy, HSTS over https.
- Per-tenant request budgets (429 with `Retry-After`), a per-address limiter on failed bearer attempts at the API,
  and brute-force detection plus login event logging in every realm.
- `/internal/*` needs the service key and is invisible outside `DISPUTE_INTERNAL_CIDRS`.
- Both servers refuse to start in production on development secrets.
- Tenant keys are stored hashed with a visible prefix, can expire, and record last use for rotation decisions.

## Not in place

MFA (a realm policy, deferred by decision) and TLS termination itself (a deployment concern; every setting assumes
https in production). Authorisation beyond tenancy exists for one surface so far: `/tenant-keys` requires an analyst
token carrying the `tenant-admin` realm role (`auth.RequireRole`); a tenant key or a plain analyst gets 403
`forbidden`. New role-gated operations follow the same pattern.

## Configuration

API: `DISPUTE_SERVICE_KEY`, `DISPUTE_INTERNAL_CIDRS`, `DISPUTE_CORS_ORIGINS`, `DISPUTE_RATE_PER_MINUTE`,
`DISPUTE_AUTH_FAILURES_PER_MINUTE`, `DISPUTE_ENV=production`. Frontend: `APP_URL`, `KEYCLOAK_URL`,
`KEYCLOAK_PUBLIC_URL`, `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET`, `DISPUTE_SERVICE_KEY`, `SESSION_SECRET`,
`SESSION_TTL_SECONDS`, `SESSION_IDLE_SECONDS`, `APP_ENV=production`. Realms: `deploy/keycloak/realm.template.json`,
rendered per tenant by `deploy/keycloak/render-realms.sh` (`APP_URL` and `KEYCLOAK_FRONTEND_SECRET`).
