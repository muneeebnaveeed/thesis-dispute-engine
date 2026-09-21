-- Tenants share the schema and never share rows. The API sets app.tenant_id per transaction; row-level security on
-- the dispute_app role hides every other tenant, and an unset setting hides everything (fail closed).
CREATE TABLE tenants (
    id          uuid PRIMARY KEY,
    name        text NOT NULL,
    settings    jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now()
);
GRANT SELECT ON tenants TO dispute_app;

-- Existing rows belong to a fixed default tenant so single-tenant deployments keep working unchanged.
INSERT INTO tenants (id, name) VALUES ('00000000-0000-8000-8000-00000000a001', 'default');

CREATE FUNCTION current_tenant_id() RETURNS uuid LANGUAGE sql STABLE AS $$
    SELECT NULLIF(current_setting('app.tenant_id', true), '')::uuid
$$;

ALTER TABLE accounts         ADD COLUMN tenant_id uuid REFERENCES tenants (id) DEFAULT current_tenant_id();
ALTER TABLE transactions     ADD COLUMN tenant_id uuid REFERENCES tenants (id) DEFAULT current_tenant_id();
ALTER TABLE disputes         ADD COLUMN tenant_id uuid REFERENCES tenants (id) DEFAULT current_tenant_id();
ALTER TABLE dispute_events   ADD COLUMN tenant_id uuid REFERENCES tenants (id) DEFAULT current_tenant_id();
ALTER TABLE idempotency_keys ADD COLUMN tenant_id uuid REFERENCES tenants (id) DEFAULT current_tenant_id();

UPDATE accounts         SET tenant_id = '00000000-0000-8000-8000-00000000a001' WHERE tenant_id IS NULL;
UPDATE transactions     SET tenant_id = '00000000-0000-8000-8000-00000000a001' WHERE tenant_id IS NULL;
UPDATE disputes         SET tenant_id = '00000000-0000-8000-8000-00000000a001' WHERE tenant_id IS NULL;
-- The append-only trigger also refuses this backfill; lift it for these statements only, inside the migration's
-- transaction, so the rows change once and the guard is back before anything else can run.
ALTER TABLE dispute_events DISABLE TRIGGER dispute_events_no_update;
UPDATE dispute_events   SET tenant_id = '00000000-0000-8000-8000-00000000a001' WHERE tenant_id IS NULL;
ALTER TABLE dispute_events ENABLE TRIGGER dispute_events_no_update;
UPDATE idempotency_keys SET tenant_id = '00000000-0000-8000-8000-00000000a001' WHERE tenant_id IS NULL;

ALTER TABLE accounts         ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE transactions     ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE disputes         ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE dispute_events   ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE idempotency_keys ALTER COLUMN tenant_id SET NOT NULL;

-- Idempotency keys are chosen by clients; two tenants may pick the same one.
ALTER TABLE idempotency_keys DROP CONSTRAINT idempotency_keys_pkey;
ALTER TABLE idempotency_keys ADD PRIMARY KEY (tenant_id, scope, key);

CREATE INDEX accounts_tenant_id_idx       ON accounts (tenant_id);
CREATE INDEX transactions_tenant_id_idx   ON transactions (tenant_id, occurred_at DESC);
CREATE INDEX disputes_tenant_id_idx       ON disputes (tenant_id, opened_at DESC);
CREATE INDEX dispute_events_tenant_id_idx ON dispute_events (tenant_id, dispute_id);

ALTER TABLE accounts         ENABLE ROW LEVEL SECURITY;
ALTER TABLE transactions     ENABLE ROW LEVEL SECURITY;
ALTER TABLE disputes         ENABLE ROW LEVEL SECURITY;
ALTER TABLE dispute_events   ENABLE ROW LEVEL SECURITY;
ALTER TABLE idempotency_keys ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation ON accounts         USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON transactions     USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON disputes         USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON dispute_events   USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
CREATE POLICY tenant_isolation ON idempotency_keys USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());

-- The only cross-tenant operations the API may perform, through owner-defined objects rather than table access.
-- search_path is pinned at creation so the body resolves to this schema's tables, not the caller's.
CREATE FUNCTION purge_idempotency_keys(before timestamptz) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path FROM CURRENT AS $$
DECLARE n bigint;
BEGIN
    DELETE FROM idempotency_keys WHERE created_at < before;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;

-- A view runs with its owner's privileges (security_invoker is off), so it reads across tenants for the gauge.
CREATE VIEW disputes_by_state AS
    SELECT tenant_id, regime, state, count(*)::bigint AS n FROM disputes GROUP BY tenant_id, regime, state;

REVOKE DELETE ON idempotency_keys FROM dispute_app;
REVOKE EXECUTE ON FUNCTION purge_idempotency_keys(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION purge_idempotency_keys(timestamptz) TO dispute_app;
GRANT SELECT ON disputes_by_state TO dispute_app;
