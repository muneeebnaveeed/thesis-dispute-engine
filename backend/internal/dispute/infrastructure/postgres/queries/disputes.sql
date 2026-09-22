-- name: InsertAccount :exec
INSERT INTO accounts (id, tenant_id, holder_name, currency, email, postal_address) VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, postal_address = EXCLUDED.postal_address;

-- name: GetAccount :one
SELECT id, holder_name, currency, email, postal_address, opened_at FROM accounts WHERE id = $1;

-- name: CountAccountDisputesSince :one
SELECT count(*)::int FROM disputes WHERE account_id = $1 AND opened_at >= $2 AND id <> $3;

-- name: CountAccountLostChargebacks :one
SELECT count(DISTINCT d.id)::int FROM disputes d
JOIN dispute_events e ON e.dispute_id = d.id
WHERE d.account_id = $1 AND d.id <> $2 AND e.to_state = 'CHARGEBACK_LOST';

-- name: InsertRiskAssessment :exec
INSERT INTO risk_assessments (dispute_id, seq, score, tier, signals, assessed_at) VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListRiskAssessments :many
SELECT id, dispute_id, seq, score, tier, signals, assessed_at FROM risk_assessments WHERE dispute_id = $1 ORDER BY id;

-- name: LatestRiskTiers :many
-- The newest assessment per dispute in the set, for list rows.
SELECT DISTINCT ON (dispute_id) dispute_id, score, tier FROM risk_assessments
WHERE dispute_id = ANY(sqlc.arg(dispute_ids)::uuid[]) ORDER BY dispute_id, id DESC;

-- name: GetTenantName :one
SELECT name FROM tenants WHERE id = current_tenant_id();

-- name: InsertTransaction :exec
INSERT INTO transactions (id, tenant_id, account_id, rail, amount, currency, merchant, occurred_at, mcc)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (id) DO UPDATE SET mcc = EXCLUDED.mcc;

-- name: GetTransaction :one
SELECT t.id, t.account_id, t.rail, t.amount, t.currency, t.merchant, t.occurred_at, t.mcc, a.currency AS account_currency, a.opened_at AS account_opened_at
FROM transactions t
JOIN accounts a ON a.id = t.account_id
WHERE t.id = $1;

-- name: InsertDispute :exec
INSERT INTO disputes (id, regime, state, appeals, version, transaction_id, account_id, disputed_amount, currency, opened_at, updated_at, reason)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $10, $11);

-- name: GetDispute :one
SELECT id, tenant_id, regime, state, appeals, version, transaction_id, account_id, disputed_amount, currency, opened_at, updated_at, reason
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
SELECT id, regime, state, transaction_id, disputed_amount, currency, opened_at, updated_at, reason
FROM disputes
WHERE (sqlc.narg(state)::text IS NULL OR state = sqlc.narg(state)::text)
  AND (NOT sqlc.arg(overdue)::boolean OR EXISTS (
        SELECT 1 FROM dispute_deadlines dl
        WHERE dl.dispute_id = disputes.id AND dl.met_at IS NULL AND dl.voided_at IS NULL AND dl.due_at < sqlc.arg(now)::timestamptz))
  AND (sqlc.narg(before_opened_at)::timestamptz IS NULL OR (opened_at, id) < (sqlc.narg(before_opened_at)::timestamptz, sqlc.narg(before_id)::uuid))
ORDER BY opened_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: InsertDeadline :exec
INSERT INTO dispute_deadlines (dispute_id, kind, cycle, started_at, due_at, basis)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListDeadlines :many
SELECT dispute_id, kind, cycle, started_at, due_at, met_at, voided_at, basis
FROM dispute_deadlines
WHERE dispute_id = $1
ORDER BY cycle, started_at, kind;

-- name: SettleDeadline :execrows
UPDATE dispute_deadlines
SET met_at = CASE WHEN sqlc.arg(met)::boolean THEN sqlc.arg(at)::timestamptz ELSE met_at END,
    voided_at = CASE WHEN sqlc.arg(met)::boolean THEN voided_at ELSE sqlc.arg(at)::timestamptz END
WHERE dispute_id = sqlc.arg(dispute_id) AND kind = sqlc.arg(kind) AND cycle = sqlc.arg(cycle) AND met_at IS NULL AND voided_at IS NULL;

-- name: NextDeadlines :many
-- The earliest open clock for each dispute in the set; a dispute with none has no row.
SELECT DISTINCT ON (dispute_id) dispute_id, kind, cycle, started_at, due_at, met_at, voided_at, basis
FROM dispute_deadlines
WHERE dispute_id = ANY(sqlc.arg(dispute_ids)::uuid[]) AND met_at IS NULL AND voided_at IS NULL
ORDER BY dispute_id, due_at;

-- name: CountDeadlinesOverdue :many
SELECT tenant_id, regime, kind, n FROM deadlines_overdue;

-- name: GetTenantCalendar :one
SELECT COALESCE(settings->>'timezone', '')::text AS timezone,
       COALESCE(ARRAY(SELECT jsonb_array_elements_text(settings->'holidays')), '{}')::text[] AS holidays
FROM tenants WHERE id = current_tenant_id();

-- name: SetTenantCalendar :execrows
-- The business-day calendar the regulatory clocks use (docs/adr/0013); other settings keys are left alone.
UPDATE tenants
SET settings = settings || jsonb_build_object('timezone', sqlc.arg(timezone)::text, 'holidays', to_jsonb(sqlc.arg(holidays)::text[]))
WHERE id = sqlc.arg(id);

-- name: InsertLedgerEntry :exec
INSERT INTO ledger_entries (dispute_id, seq, kind, debit_account, credit_account, amount, currency, reference, posted_at,
                            core_rrn, core_response_code, core_latency_ms)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ListLedgerEntries :many
SELECT id, dispute_id, seq, kind, debit_account, credit_account, amount, currency, reference, posted_at,
       core_rrn, core_response_code, core_latency_ms
FROM ledger_entries
WHERE dispute_id = $1
ORDER BY id;

-- name: SuspenseByRegime :many
SELECT tenant_id, regime, currency, balance::numeric AS balance FROM suspense_by_regime;

-- name: GetTenantCore :one
SELECT COALESCE(settings->'core'->>'kind', '')::text AS kind, COALESCE(settings->'core', '{}'::jsonb)::jsonb AS settings
FROM tenants WHERE id = current_tenant_id();

-- name: SetTenantCore :execrows
UPDATE tenants SET settings = settings || jsonb_build_object('core', sqlc.arg(core)::jsonb) WHERE id = sqlc.arg(id);

-- name: UpsertQuestionnaire :exec
-- Sending again (after an appeal, say) asks afresh: the questions are replaced and any answers cleared.
INSERT INTO questionnaires (dispute_id, reason, questions, sent_at) VALUES ($1, $2, $3, $4)
ON CONFLICT (dispute_id) DO UPDATE SET reason = EXCLUDED.reason, questions = EXCLUDED.questions, sent_at = EXCLUDED.sent_at,
  answers = NULL, received_at = NULL;

-- name: AnswerQuestionnaire :execrows
UPDATE questionnaires SET answers = $2, received_at = $3 WHERE dispute_id = $1 AND received_at IS NULL;

-- name: GetQuestionnaire :one
SELECT dispute_id, reason, questions, answers, sent_at, received_at FROM questionnaires WHERE dispute_id = $1;

-- name: InsertNotice :one
INSERT INTO notices (dispute_id, seq, kind, channel, recipient, subject, document, created_at, sent_at, actor, resend_of)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
RETURNING id;

-- name: ListNotices :many
SELECT id, dispute_id, seq, kind, channel, recipient, subject, document, created_at, sent_at, attempts, last_error, actor, resend_of
FROM notices WHERE dispute_id = $1 ORDER BY id;

-- name: GetNotice :one
SELECT id, dispute_id, seq, kind, channel, recipient, subject, document, created_at, sent_at, attempts, last_error, actor, resend_of
FROM notices WHERE id = $1 AND dispute_id = $2;

-- name: ClaimNotices :many
SELECT id, tenant_id, dispute_id, seq, kind, channel, recipient, subject, document, created_at, attempts FROM claim_notices($1);

-- name: FinishNotice :exec
SELECT finish_notice(sqlc.arg(notice_id), sqlc.narg(failure)::text);
