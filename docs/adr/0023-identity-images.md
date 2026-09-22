# 0023: Tenant logos and analyst pictures are small bytea rows served through the workbench

**Status:** accepted, 2026-09-22; builds on ADR 0005 and 0008

## Context

The sidebar had no identity: it wrote "Dispute Engine" over the tenant slug, and it named the
signed-in analyst in a sign-out label. An analyst who works two disputes a minute should be able to
tell at a glance which organisation the window belongs to, and see themselves in it. That needs two
uploaded images: one per tenant, one per person.

Uploaded images usually mean object storage, and object storage means a second stateful dependency,
a bucket policy per tenant, presigned URLs and a lifecycle rule for orphans. The images here are
drawn at 40 pixels. A generous cap of 256 kB makes the whole corpus for a hundred tenants smaller
than one day of the event log.

## Decision

- Two tables: `tenant_metadata`, one row per tenant carrying the logo, its content type and the time
  it was replaced; `analyst_profiles`, keyed by `(tenant_id, subject)`, carrying the picture. Both
  default `tenant_id` to `current_tenant_id()` and are covered by the same row-level security policy
  as every other tenant-owned table, so an image cannot leak across organisations by identifier.
  `tenant_metadata` is named for what it will hold next (accent colour, sender display name), not
  for the logo alone.
- The bytes live in `bytea` with a `CHECK` cap of 256 kB, alongside a `CHECK` that a logo and its
  content type are either both present or both absent.
- PNG, JPEG and WebP only. SVG is refused: it is a document that can carry script, and it would be
  served from the API's own origin. The refusal is the `image-refused` code, a validation failure in
  the client taxonomy (ADR 0012), so the sidebar can show the reason without a page of its own.
- `GET /tenant` answers with the name and whether a logo exists; `GET /tenant/logo` and
  `GET /me/avatar` serve the bytes under their own content type with `X-Content-Type-Options:
  nosniff`. Replacing is `PUT` multipart: the logo needs the `tenant-admin` role, the picture belongs
  to whoever is signed in and needs no role, but is refused for a tenant key, which has no person
  behind it.
- The workbench reads both through server functions (ADR 0020) and inlines them as data URLs. A bare
  URL in `src/` would not carry the analyst's session, and a proxy route would exist only to add the
  cookie; at 256 kB the inline form costs less than the round trip it saves.
- `cmd/seed` embeds a plain wordmark for each development tenant so the sidebar has something to draw
  before anyone uploads anything.

## Consequences

- Easier: no second stateful dependency and no bucket lifecycle; tenant isolation is the policy that
  already exists; a logo travels with a `pg_dump` of the tenant, which is what the operations runbook
  restores.
- Harder: images move through the API process and the query path rather than a CDN, so the cap is
  load-bearing and a future gallery of larger attachments would not fit this shape; data URLs mean
  the picture is re-sent whenever its query is refetched rather than cached by the browser as a file.
