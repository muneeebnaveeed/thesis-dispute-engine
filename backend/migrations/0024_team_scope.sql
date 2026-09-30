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
-- reassign moves a dispute out of the caller's teams; Cedar decides it, so the new row is not checked here
CREATE POLICY team_scope_update ON disputes AS RESTRICTIVE FOR UPDATE USING (in_team(tenant_id, team)) WITH CHECK (true);

CREATE POLICY team_scope ON dispute_events    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON dispute_deadlines AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON ledger_entries    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON questionnaires    AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON notices           AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON risk_assessments  AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
CREATE POLICY team_scope ON attachments       AS RESTRICTIVE USING (EXISTS (SELECT 1 FROM disputes d WHERE d.id = dispute_id));
