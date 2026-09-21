-- Tenant tenant keys. The secret is shown once at creation; only its SHA-256 is stored. Not under row-level security:
-- the lookup is what establishes the tenant, so it runs before one is known, and the table holds no tenant data.
CREATE TABLE tenant_keys (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants (id),
    key_hash    bytea NOT NULL UNIQUE,
    label       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz
);

CREATE INDEX tenant_keys_tenant_id_idx ON tenant_keys (tenant_id);

GRANT SELECT ON tenant_keys TO dispute_app;
