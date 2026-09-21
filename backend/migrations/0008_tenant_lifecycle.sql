-- Tenant lifecycle and sign-in discovery: a disabled tenant's credentials stop working immediately; email domains let
-- the sign-in page find a tenant's realm from a work address; sessions carry subject and realm session id so
-- back-channel logout and user offboarding can end them.
ALTER TABLE tenants ADD COLUMN disabled_at   timestamptz;
ALTER TABLE tenants ADD COLUMN email_domains text[] NOT NULL DEFAULT '{}';
CREATE INDEX tenants_email_domains_idx ON tenants USING gin (email_domains);

ALTER TABLE web_sessions ADD COLUMN subject text;
ALTER TABLE web_sessions ADD COLUMN sid     text;
CREATE INDEX web_sessions_subject_idx ON web_sessions (tenant_id, subject);
CREATE INDEX web_sessions_sid_idx     ON web_sessions (sid);

-- Per-tenant request budget (ADR 0007): one row per tenant per minute, bumped in place.
CREATE TABLE tenant_rate_windows (
    tenant_id     uuid NOT NULL REFERENCES tenants (id),
    window_start  timestamptz NOT NULL,
    count         int NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, window_start)
);
GRANT SELECT, INSERT, UPDATE, DELETE ON tenant_rate_windows TO dispute_app;
