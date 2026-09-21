-- dispute_app is the privilege set the API runs with. It is a NOLOGIN group role so no secret lives in a migration;
-- each environment creates a LOGIN role (dispute_api locally and in CI, see deploy/postgres/roles.sql) and joins it here.
DO $$
BEGIN
    BEGIN
        CREATE ROLE dispute_app NOLOGIN;
    EXCEPTION WHEN duplicate_object THEN
        NULL;
    END;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dispute_api') THEN
        GRANT dispute_app TO dispute_api;
    END IF;
    -- Schema name is not known statically: tests migrate into a throwaway schema, deployments into public.
    EXECUTE format('GRANT USAGE ON SCHEMA %I TO dispute_app', current_schema());
    EXECUTE format('GRANT USAGE ON ALL SEQUENCES IN SCHEMA %I TO dispute_app', current_schema());
END $$;

GRANT SELECT                 ON schema_migrations TO dispute_app;
GRANT SELECT, INSERT, UPDATE ON accounts          TO dispute_app;
GRANT SELECT, INSERT, UPDATE ON transactions      TO dispute_app;
GRANT SELECT, INSERT, UPDATE ON disputes          TO dispute_app;
-- The log is append-only twice over: the trigger stops the owner, the missing privilege stops the API.
GRANT SELECT, INSERT         ON dispute_events    TO dispute_app;
GRANT SELECT, INSERT, DELETE ON idempotency_keys  TO dispute_app;
