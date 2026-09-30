-- A second, independent line behind the Cedar checks: an analyst sees only disputes of teams they belong to. These
-- policies are RESTRICTIVE because permissive policies are ORed with tenant_isolation and would widen it.
CREATE FUNCTION current_subject() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.subject', true), '')
$$;
CREATE FUNCTION current_principal_kind() RETURNS text LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.principal_kind', true), '')
$$;
CREATE FUNCTION in_team(t uuid, team text) RETURNS boolean LANGUAGE sql STABLE AS $$
    SELECT current_principal_kind() = 'key'
        OR EXISTS (SELECT 1 FROM team_members m WHERE m.tenant_id = t AND m.team = in_team.team AND m.subject = current_subject())
$$;

CREATE POLICY team_scope_read   ON disputes AS RESTRICTIVE FOR SELECT USING (in_team(tenant_id, team));
CREATE POLICY team_scope_insert ON disputes AS RESTRICTIVE FOR INSERT WITH CHECK (in_team(tenant_id, team));
CREATE POLICY team_scope_update ON disputes AS RESTRICTIVE FOR UPDATE USING (in_team(tenant_id, team));

-- Reassigning moves a dispute out of the caller's teams, which no policy above can allow: an UPDATE's new row must
-- also pass the SELECT policies. The move runs as the owner, still bounded to the caller's tenant, to a dispute the
-- caller can see now, and to the version they read; whether they may move it at all is Cedar's decision.
CREATE FUNCTION reassign_dispute(dispute uuid, expected bigint, new_team text, at timestamptz) RETURNS bigint
LANGUAGE sql SECURITY DEFINER SET search_path FROM CURRENT AS $$
    WITH moved AS (
        UPDATE disputes SET team = new_team, version = version + 1, updated_at = at
        WHERE id = dispute AND version = expected AND tenant_id = current_tenant_id() AND in_team(tenant_id, team)
        RETURNING 1
    )
    SELECT count(*) FROM moved
$$;
REVOKE EXECUTE ON FUNCTION reassign_dispute(uuid, bigint, text, timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION reassign_dispute(uuid, bigint, text, timestamptz) TO dispute_app;

CREATE POLICY team_scope ON dispute_events    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON dispute_deadlines AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON ledger_entries    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON questionnaires    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON notices           AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON risk_assessments  AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON attachments       AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
