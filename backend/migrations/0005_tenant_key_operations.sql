-- Operating tenant keys: identify a leaked one by its visible prefix, see whether a key is still in use before
-- revoking it, and let time-boxed keys expire on their own.
ALTER TABLE tenant_keys ADD COLUMN prefix       text NOT NULL DEFAULT '';
ALTER TABLE tenant_keys ADD COLUMN last_used_at timestamptz;
ALTER TABLE tenant_keys ADD COLUMN expires_at   timestamptz;
ALTER TABLE tenant_keys ALTER COLUMN prefix DROP DEFAULT;

CREATE INDEX tenant_keys_prefix_idx ON tenant_keys (prefix);

-- The API may record use, and nothing else about a key.
GRANT UPDATE (last_used_at) ON tenant_keys TO dispute_app;
