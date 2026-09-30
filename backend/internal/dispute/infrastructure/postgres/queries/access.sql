-- name: GetPolicyVersion :one
SELECT COALESCE((SELECT version FROM policy_versions), 0)::bigint AS version;

-- name: ListTeams :many
SELECT slug, is_default FROM teams ORDER BY slug;

-- name: ListRoleGrants :many
SELECT team, role, action, amount_limit FROM role_grants ORDER BY team, role, action;

-- name: ListMemberships :many
SELECT team, role FROM team_members WHERE subject = $1 ORDER BY team;

-- name: ListRoutingRules :many
SELECT position, rail, reason, risk_tier, min_amount, team FROM routing_rules ORDER BY position;
