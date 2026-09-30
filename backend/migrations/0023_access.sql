-- Who may do what inside a tenant is data, so a tenant admin can later edit it; the rules no tenant may change are
-- Cedar in the binary (ADR 0027). Every tenant starts with one default team that owns all existing disputes.
CREATE TABLE teams (
    tenant_id  uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    slug       text NOT NULL CHECK (slug ~ '^[a-z][a-z0-9-]{0,31}$'),
    name       text NOT NULL,
    is_default boolean NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, slug)
);
CREATE UNIQUE INDEX teams_one_default ON teams (tenant_id) WHERE is_default;

CREATE TABLE team_members (
    tenant_id uuid NOT NULL DEFAULT current_tenant_id(),
    team      text NOT NULL,
    subject   text NOT NULL,
    role      text NOT NULL CHECK (role IN ('junior', 'senior', 'lead')),
    PRIMARY KEY (tenant_id, team, subject),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);
CREATE INDEX team_members_subject_idx ON team_members (tenant_id, subject);

CREATE TABLE role_grants (
    tenant_id    uuid NOT NULL DEFAULT current_tenant_id(),
    team         text NOT NULL,
    role         text NOT NULL CHECK (role IN ('junior', 'senior', 'lead')),
    action       text NOT NULL CHECK (action ~ '^[A-Za-z0-9_-]{1,64}$'),
    amount_limit numeric(19, 4) CHECK (amount_limit IS NULL OR amount_limit >= 0),
    PRIMARY KEY (tenant_id, team, role, action),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);

CREATE TABLE routing_rules (
    tenant_id  uuid NOT NULL DEFAULT current_tenant_id(),
    position   int NOT NULL,
    rail       text,
    reason     text,
    risk_tier  text,
    min_amount numeric(19, 4),
    team       text NOT NULL,
    PRIMARY KEY (tenant_id, position),
    FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug) ON DELETE CASCADE
);

CREATE TABLE policy_versions (
    tenant_id uuid PRIMARY KEY REFERENCES tenants (id) DEFAULT current_tenant_id(),
    version   bigint NOT NULL DEFAULT 1
);

-- any change to a tenant's access data invalidates its compiled policy set on the next request
CREATE FUNCTION bump_policy_version() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE t uuid;
BEGIN
    IF TG_OP = 'DELETE' THEN t := OLD.tenant_id; ELSE t := NEW.tenant_id; END IF;
    INSERT INTO policy_versions (tenant_id, version) VALUES (t, 1)
    ON CONFLICT (tenant_id) DO UPDATE SET version = policy_versions.version + 1;
    RETURN NULL;
END $$;
CREATE TRIGGER teams_version AFTER INSERT OR UPDATE OR DELETE ON teams FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER team_members_version AFTER INSERT OR UPDATE OR DELETE ON team_members FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER role_grants_version AFTER INSERT OR UPDATE OR DELETE ON role_grants FOR EACH ROW EXECUTE FUNCTION bump_policy_version();
CREATE TRIGGER routing_rules_version AFTER INSERT OR UPDATE OR DELETE ON routing_rules FOR EACH ROW EXECUTE FUNCTION bump_policy_version();

-- every tenant, existing and future, has the default team and today's grants on it
CREATE FUNCTION default_access(t uuid) RETURNS void LANGUAGE sql AS $$
    INSERT INTO teams (tenant_id, slug, name, is_default) VALUES (t, 'general', 'General', true);
    INSERT INTO role_grants (tenant_id, team, role, action)
    SELECT t, 'general', 'junior', a FROM unnest(ARRAY[
        'create', 'compose-email', 'upload-attachment', 'resend-notice',
        'OPEN_INVESTIGATION', 'SEND_QUESTIONNAIRE', 'RECEIVE_QUESTIONNAIRE', 'ISSUE_REFUND', 'FILE_CHARGEBACK',
        'ACKNOWLEDGE_CHARGEBACK', 'SUBMIT_EVIDENCE', 'WIN_CHARGEBACK', 'LOSE_CHARGEBACK', 'ISSUE_FINAL_CREDIT',
        'REVERSE_PROVISIONAL_CREDIT', 'CLOSE', 'APPEAL']) AS a;
    INSERT INTO role_grants (tenant_id, team, role, action) VALUES (t, 'general', 'lead', 'reassign');
$$;
SELECT default_access(id) FROM tenants;
CREATE FUNCTION tenants_default_access() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN PERFORM default_access(NEW.id); RETURN NULL; END $$;
CREATE TRIGGER tenants_default_access AFTER INSERT ON tenants FOR EACH ROW EXECUTE FUNCTION tenants_default_access();

ALTER TABLE disputes ADD COLUMN team text;
UPDATE disputes SET team = 'general';
ALTER TABLE disputes ALTER COLUMN team SET NOT NULL;
ALTER TABLE disputes ADD FOREIGN KEY (tenant_id, team) REFERENCES teams (tenant_id, slug);
CREATE INDEX disputes_team_idx ON disputes (tenant_id, team, opened_at DESC);

ALTER TABLE teams           ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_members    ENABLE ROW LEVEL SECURITY;
ALTER TABLE role_grants     ENABLE ROW LEVEL SECURITY;
ALTER TABLE routing_rules   ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_versions ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON teams           USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON team_members    USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON role_grants     USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON routing_rules   USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON policy_versions USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT ON teams, team_members, role_grants, routing_rules, policy_versions TO dispute_app;
