-- Tenant admins manage their own keys from the UI (ADR 0009 follow-up): the API may issue and revoke keys, never
-- relabel or unrevoke them, and the tenant scope is enforced in every query.
GRANT INSERT ON tenant_keys TO dispute_app;
GRANT UPDATE (revoked_at) ON tenant_keys TO dispute_app;
