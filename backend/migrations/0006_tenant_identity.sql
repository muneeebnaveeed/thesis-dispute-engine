-- Analysts sign in through one Keycloak realm per tenant; a token's issuer identifies the tenant. slug names the
-- realm and the tenant's URL segment in the frontend.
ALTER TABLE tenants ADD COLUMN slug        text;
ALTER TABLE tenants ADD COLUMN oidc_issuer text;

UPDATE tenants SET slug = 'alpha', name = 'Alpha Bank' WHERE id = '00000000-0000-8000-8000-00000000a001' AND slug IS NULL;
UPDATE tenants SET slug = replace(id::text, '-', '') WHERE slug IS NULL;

ALTER TABLE tenants ALTER COLUMN slug SET NOT NULL;
ALTER TABLE tenants ADD CONSTRAINT tenants_slug_key UNIQUE (slug);
ALTER TABLE tenants ADD CONSTRAINT tenants_slug_shape CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,62}$');
ALTER TABLE tenants ADD CONSTRAINT tenants_oidc_issuer_key UNIQUE (oidc_issuer);
