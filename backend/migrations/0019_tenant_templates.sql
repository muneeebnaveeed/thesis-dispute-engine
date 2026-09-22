-- A tenant's own wording for an analyst email template (docs/adr/0019): the words and option texts change,
-- the form (field ids and types) stays the base's so the workbench and the validator keep one contract.
CREATE TABLE tenant_templates (
    tenant_id   uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    kind        text NOT NULL,
    override    jsonb NOT NULL,            -- notice.Override
    updated_by  text NOT NULL,
    updated_at  timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, kind)
);

ALTER TABLE tenant_templates ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_templates USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON tenant_templates TO dispute_app;
