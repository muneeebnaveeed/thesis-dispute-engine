-- Tenant API keys. The secret is shown once at creation; only its SHA-256 is stored. Not under row-level security:
-- the lookup is what establishes the tenant, so it runs before one is known, and the table holds no tenant data.
CREATE TABLE api_keys (
    id          uuid PRIMARY KEY,
    tenant_id   uuid NOT NULL REFERENCES tenants (id),
    key_hash    bytea NOT NULL UNIQUE,
    label       text NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz
);

CREATE INDEX api_keys_tenant_id_idx ON api_keys (tenant_id);

GRANT SELECT ON api_keys TO dispute_app;
