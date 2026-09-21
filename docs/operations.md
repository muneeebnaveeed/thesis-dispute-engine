# Operations

Operator tasks that are not part of a request. All of them run as the schema owner
(`DISPUTE_MIGRATE_DATABASE_URL`) from `backend/`.

## Tenant keys

A tenant key is the credential a customer's own systems present on every request
(`Authorization: Bearer tk_...`). The server stores its SHA-256 and the first 12 characters; the
full value exists only at the moment of creation.

```sh
go run ./cmd/tenantkey create --tenant <tenant uuid> --label "core banking prod" [--expires 2027-03-01]
go run ./cmd/tenantkey list
go run ./cmd/tenantkey find tk_j0zS9zjIMD...        # a leaked value or just its prefix
go run ./cmd/tenantkey revoke --id <uuid> | --prefix tk_j0zS9zjI
```

Revocation and expiry take effect on the next request; nothing is cached.

### Issuing a customer's first key

Issue two: a primary and a standby with different labels. Hand both over through the customer's
secret channel. If the primary ever leaks, they switch to the standby themselves and you revoke
the primary; no one waits on anyone.

### Rotating a key (planned)

1. `create` a new key for the tenant; deliver it.
2. The customer switches their configuration to it.
3. Watch `list`: when the old key's `last used` stops advancing (it updates at most once a minute
   while in use), `revoke` it.

### A key leaked

1. `find <the leaked value>` to learn which row it is. Prefix alone is enough if that is all that
   leaked.
2. `revoke --id <that id>` immediately; the customer's integration fails closed with 401 from the
   next request.
3. `create` a replacement and deliver it. Review the dashboard's `Problems by code` panel for
   `unauthenticated` from the leaked key's last known caller if you want to know whether it was
   used.

### Time-boxed access

`--expires` makes a key die on its own; use it for migration windows, auditors and demos.

### Rotation reminders

`go run ./cmd/tenantkey audit [--stale 180] [--idle 30]` lists live keys older than the stale threshold or unused
for longer than the idle threshold and exits non-zero when it finds any; run it from a daily cron and treat a
failure as a ticket.

## Tenants

```sh
go run ./cmd/tenant upsert --slug otp --name "OTP Bank" --domains otpbank.hu        # row, issuer, email domains
go run ./cmd/tenant disable --slug otp   # every key and analyst token stops resolving on the next request; data kept
go run ./cmd/tenant enable  --slug otp
go run ./cmd/tenant list
```

`scripts/tenant` wraps these together with the Keycloak realm and the first keys.

## Onboarding a tenant

```sh
scripts/tenant onboard --slug otp --name "OTP Bank" --domains otpbank.hu
```

In order: the tenant row (id, slug, issuer, email domains), the realm rendered from the template and created
in Keycloak (brute-force detection, login events, back-channel logout and the frontend client come with it),
and two tenant keys labelled primary and standby, printed once. The summary gives the front door,
`$APP_URL/<slug>`, which is all an analyst ever needs. Deliver the keys over the customer's secret channel.
Re-running is safe: an existing realm is left alone and only new keys are added (pass `--no-keys` to skip them).

## Analysts

```sh
scripts/tenant user add    --slug otp --username jane --email jane@otpbank.hu --name "Jane Kovacs" --roles analyst,tenant-admin
scripts/tenant user list   --slug otp
scripts/tenant user remove --slug otp --username jane
```

`add` creates the account with a temporary password (printed once) that must be changed at first sign-in;
until then direct token grants are refused ("account is not fully set up"), which is correct. `remove`
disables the account, has Keycloak log the user out (which reaches us through back-channel logout) and ends
any remaining session directly. Nothing is deleted, so the audit trail keeps the name.

## Offboarding a tenant

```sh
scripts/tenant offboard --slug otp        # asks for confirmation; --yes to skip
```

Disables the tenant row (every key and analyst token stops resolving on the next request), revokes every live
key, ends every analyst session, and disables the realm. Data is retained for the retention obligation;
`cmd/tenant enable` plus re-enabling the realm in Keycloak reverses it.

## Production settings the API insists on

With `DISPUTE_ENV=production` the API refuses to start on dev defaults: a `DISPUTE_SERVICE_KEY` shorter than 32
characters or equal to the dev value, the dev database credentials, or `sslmode=disable`. `/internal/*` is
invisible (404) to peers outside `DISPUTE_INTERNAL_CIDRS`; keep it off the public ingress as well. Per-tenant
request budgets (`DISPUTE_RATE_PER_MINUTE`, default 600) answer 429 with `Retry-After`. There is no CDN or
edge WAF in this deployment by decision, so the application is the whole line: per-tenant budgets here, a
per-IP limiter on unauthenticated failures at the API, and brute-force detection in each Keycloak realm.
