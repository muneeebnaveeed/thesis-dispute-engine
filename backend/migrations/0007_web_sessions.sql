-- Opaque session store for the frontend server (ADR 0011): the API persists a ciphertext it cannot read, keyed by
-- session id, and sweeps expired rows. Not under row-level security: a session exists before its tenant is known.
CREATE TABLE web_sessions (
    id          uuid PRIMARY KEY,
    tenant_id   uuid REFERENCES tenants (id),
    ciphertext  bytea NOT NULL,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX web_sessions_expires_at_idx ON web_sessions (expires_at);

GRANT SELECT, INSERT, UPDATE, DELETE ON web_sessions TO dispute_app;
