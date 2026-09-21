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
