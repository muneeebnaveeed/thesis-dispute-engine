-- What the workbench shows about a tenant beyond its name. One row per tenant, created on first write; for now it
-- holds a logo, which is why the table is named for metadata rather than for the logo.
CREATE TABLE tenant_metadata (
    tenant_id         uuid PRIMARY KEY REFERENCES tenants (id) DEFAULT current_tenant_id(),
    logo              bytea,
    logo_content_type text,
    logo_updated_at   timestamptz,
    CONSTRAINT logo_complete CHECK ((logo IS NULL) = (logo_content_type IS NULL)),
    CONSTRAINT logo_size CHECK (logo IS NULL OR octet_length(logo) <= 262144)
);
ALTER TABLE tenant_metadata ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_metadata USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON tenant_metadata TO dispute_app;

-- An analyst's own picture. Identity stays in Keycloak; this row is only what the workbench draws next to a name,
-- keyed by the subject claim so it survives a rename.
CREATE TABLE analyst_profiles (
    tenant_id    uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    subject      text NOT NULL,
    avatar       bytea NOT NULL,
    content_type text NOT NULL,
    updated_at   timestamptz NOT NULL,
    PRIMARY KEY (tenant_id, subject),
    CONSTRAINT avatar_size CHECK (octet_length(avatar) <= 262144)
);
ALTER TABLE analyst_profiles ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON analyst_profiles USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON analyst_profiles TO dispute_app;
