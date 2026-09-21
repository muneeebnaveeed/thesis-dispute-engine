-- name: InsertAccount :exec
INSERT INTO accounts (id, tenant_id, holder_name, currency) VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO NOTHING;

-- name: InsertTransaction :exec
INSERT INTO transactions (id, tenant_id, account_id, rail, amount, currency, merchant, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO NOTHING;

-- name: GetTransaction :one
SELECT t.id, t.account_id, t.rail, t.amount, t.currency, t.merchant, t.occurred_at, a.currency AS account_currency
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.id = $1;

-- name: InsertDispute :exec
INSERT INTO disputes (id, regime, state, appeals, version, transaction_id, account_id, disputed_amount, currency, opened_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10);

-- name: GetDispute :one
SELECT id, tenant_id, regime, state, appeals, version, transaction_id, account_id, disputed_amount, currency, opened_at, updated_at
FROM disputes
WHERE id = $1;

-- name: UpdateDisputeState :execrows
UPDATE disputes
SET state = $2, appeals = $3, version = version + 1, updated_at = $4
WHERE id = $1 AND version = $5;

-- name: InsertDisputeEvent :one
INSERT INTO dispute_events (dispute_id, seq, event, from_state, to_state, actor, payload, idempotency_key, trace_id, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
RETURNING id;

-- name: ListDisputeEvents :many
SELECT id, dispute_id, seq, event, from_state, to_state, actor, payload, idempotency_key, trace_id, occurred_at
FROM dispute_events
WHERE dispute_id = $1
ORDER BY seq;

-- name: GetIdempotencyKey :one
SELECT scope, key, request_hash, status_code, response, created_at
FROM idempotency_keys
WHERE scope = $1 AND key = $2;

-- name: InsertIdempotencyKey :exec
INSERT INTO idempotency_keys (scope, key, request_hash, status_code, response)
VALUES ($1, $2, $3, $4, $5);

-- name: PurgeIdempotencyKeys :one
SELECT purge_idempotency_keys($1)::bigint AS n;

-- name: CountDisputesByState :many
SELECT tenant_id, regime, state, n FROM disputes_by_state;

-- name: InsertTenantKey :exec
INSERT INTO tenant_keys (id, tenant_id, key_hash, prefix, label, expires_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetTenantByTenantKeyHash :one
SELECT k.id, k.tenant_id FROM tenant_keys k
JOIN tenants t ON t.id = k.tenant_id AND t.disabled_at IS NULL
WHERE k.key_hash = $1 AND k.revoked_at IS NULL AND (k.expires_at IS NULL OR k.expires_at > now());

-- name: TouchTenantKey :exec
UPDATE tenant_keys SET last_used_at = now() WHERE id = $1;

-- name: RevokeTenantKey :execrows
UPDATE tenant_keys SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: FindTenantKeysByPrefix :many
SELECT id, tenant_id, label, revoked_at FROM tenant_keys WHERE prefix = $1;

-- name: FindTenantKeyByHash :one
SELECT id, tenant_id, label, revoked_at FROM tenant_keys WHERE key_hash = $1;

-- name: ListTenantKeys :many
SELECT id, tenant_id, prefix, label, created_at, last_used_at, expires_at, revoked_at FROM tenant_keys ORDER BY created_at;

-- name: UpsertTenant :exec
INSERT INTO tenants (id, name, slug, oidc_issuer) VALUES ($1, $2, $3, $4)
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, slug = EXCLUDED.slug, oidc_issuer = EXCLUDED.oidc_issuer;

-- name: GetTenantByIssuer :one
SELECT id, slug FROM tenants WHERE oidc_issuer = $1 AND disabled_at IS NULL;

-- name: ListTenants :many
SELECT id, name, slug, oidc_issuer, disabled_at FROM tenants ORDER BY name;

-- name: PutWebSession :exec
INSERT INTO web_sessions (id, tenant_id, ciphertext, expires_at, subject, sid)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET tenant_id = EXCLUDED.tenant_id, ciphertext = EXCLUDED.ciphertext, expires_at = EXCLUDED.expires_at,
  subject = EXCLUDED.subject, sid = EXCLUDED.sid, updated_at = now();

-- name: GetWebSession :one
SELECT id, tenant_id, ciphertext, expires_at FROM web_sessions WHERE id = $1 AND expires_at > now();

-- name: DeleteWebSession :execrows
DELETE FROM web_sessions WHERE id = $1;

-- name: PurgeWebSessions :execrows
DELETE FROM web_sessions WHERE expires_at < now();

-- name: BumpTenantRateWindow :one
INSERT INTO tenant_rate_windows (tenant_id, window_start, count) VALUES ($1, $2, 1)
ON CONFLICT (tenant_id, window_start) DO UPDATE SET count = tenant_rate_windows.count + 1
RETURNING count;

-- name: PurgeTenantRateWindows :execrows
DELETE FROM tenant_rate_windows WHERE window_start < $1;

-- name: ListTenantsForDiscovery :many
SELECT id, name, slug, oidc_issuer, email_domains FROM tenants WHERE disabled_at IS NULL ORDER BY name;

-- name: SetTenantDisabled :execrows
UPDATE tenants SET disabled_at = CASE WHEN sqlc.arg(disabled)::boolean THEN COALESCE(disabled_at, now()) ELSE NULL END WHERE id = sqlc.arg(id);

-- name: SetTenantEmailDomains :execrows
UPDATE tenants SET email_domains = $2 WHERE id = $1;

-- name: DeleteWebSessionsBySubject :execrows
DELETE FROM web_sessions WHERE tenant_id = $1 AND subject = $2;

-- name: DeleteWebSessionsBySid :execrows
DELETE FROM web_sessions WHERE sid = $1;

-- name: DeleteWebSessionsByTenant :execrows
DELETE FROM web_sessions WHERE tenant_id = $1;

-- name: ListTenantKeysByTenant :many
SELECT id, prefix, label, created_at, last_used_at, expires_at, revoked_at FROM tenant_keys WHERE tenant_id = $1 ORDER BY created_at;

-- name: RevokeTenantKeyForTenant :execrows
UPDATE tenant_keys SET revoked_at = now() WHERE id = $1 AND tenant_id = $2 AND revoked_at IS NULL;

-- name: GetTenantKeyForTenant :one
SELECT id, prefix, label, created_at, last_used_at, expires_at, revoked_at FROM tenant_keys WHERE id = $1 AND tenant_id = $2;
-- name: ListDisputes :many
-- Keyset pagination on (opened_at, id) descending; row-level security scopes the tenant.
SELECT id, regime, state, transaction_id, disputed_amount, currency, opened_at, updated_at
FROM disputes
WHERE (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
  AND (sqlc.narg(before_opened_at)::timestamptz IS NULL OR (opened_at, id) < (sqlc.narg(before_opened_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(page_size);
