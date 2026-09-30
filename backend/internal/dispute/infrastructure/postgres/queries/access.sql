-- name: GetAccess :one
-- One round trip for everything a decision needs; RLS limits every table to the tenant.
SELECT
    COALESCE((SELECT version FROM policy_versions), 0)::bigint AS version,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('slug', slug, 'default', is_default) ORDER BY slug) FROM teams), '[]')::jsonb AS teams,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('team', team, 'role', role, 'action', action, 'limit', amount_limit::text)
        ORDER BY team, role, action) FROM role_grants), '[]')::jsonb AS grants,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('rail', rail, 'reason', reason, 'tier', risk_tier, 'min', min_amount::text, 'team', team)
        ORDER BY position) FROM routing_rules), '[]')::jsonb AS routing,
    COALESCE((SELECT jsonb_agg(jsonb_build_object('team', team, 'role', role) ORDER BY team)
        FROM team_members WHERE subject = sqlc.arg(subject)::text), '[]')::jsonb AS members;
