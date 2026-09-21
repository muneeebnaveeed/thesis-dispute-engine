-- name: InsertTenant :exec
INSERT INTO tenants (id, name) VALUES ($1, $2)
ON CONFLICT (id) DO NOTHING;

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
