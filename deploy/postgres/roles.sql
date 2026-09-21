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

-- Keycloak keeps its own state in a separate database under its own role.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'keycloak') THEN
        CREATE ROLE keycloak LOGIN PASSWORD 'keycloak';
    END IF;
END $$;
SELECT 'CREATE DATABASE keycloak OWNER keycloak' WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'keycloak') \gexec
