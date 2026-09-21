-- Login identity for the API in local and CI databases; idempotent. Production creates its own with a real secret.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dispute_api') THEN
        CREATE ROLE dispute_api LOGIN PASSWORD 'dispute_api';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'dispute_app') THEN
        GRANT dispute_app TO dispute_api;
    END IF;
END $$;
