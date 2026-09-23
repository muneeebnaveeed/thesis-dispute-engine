---
name: tenant-ops
description: Onboard or offboard a tenant, add or remove an analyst, issue or revoke tenant keys. Use when asked to set up a customer, a user, or a key.
---

Use the operator scripts; never edit tenants, realms, keys or users by hand.

- Tenant: `scripts/tenant onboard --slug <slug> --name "<name>" [--domains a.com,b.com]`,
  `scripts/tenant offboard --slug <slug> --yes`. Row, realm, first keys and their reversal in one step each.
- Analyst: `scripts/tenant user add|remove|list --slug <slug> ...`. Add prints a temporary password once.
- Keys alone: `cd backend && go run ./cmd/tenantkey create|list|find|revoke|audit`.
- Row alone: `cd backend && go run ./cmd/tenant upsert|disable|enable|list`.
- Everything needs Postgres and Keycloak up (`make auth:up`) and the owner database URL; secrets printed by
  these commands are shown once and must not be pasted into commits, PRs or notes.
- Runbooks with the reasoning: `docs/operations.md`, `docs/authentication.md`.
