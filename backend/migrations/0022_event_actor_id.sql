-- The actor used to be whatever the request body said. It is now the authenticated principal: actor stays the
-- readable name, actor_id is the stable identity policies compare (the OIDC subject or the tenant key id).
ALTER TABLE dispute_events ADD COLUMN actor_id text;
