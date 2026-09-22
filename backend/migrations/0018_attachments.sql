-- Files an analyst attaches to an email (docs/adr/0019). Bytes live in the database so the API stays the only
-- stateful client; the size cap keeps that sane. An upload is a draft until a notice claims it; drafts older
-- than a day are swept.
CREATE TABLE attachments (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id    uuid NOT NULL REFERENCES disputes (id),
    notice_id     bigint REFERENCES notices (id),          -- NULL while a draft
    filename      text NOT NULL,
    content_type  text NOT NULL,
    size          int  NOT NULL CHECK (size > 0 AND size <= 5242880),
    content       bytea NOT NULL,
    uploaded_by   text NOT NULL,
    uploaded_at   timestamptz NOT NULL
);

CREATE INDEX attachments_notice_idx ON attachments (notice_id);
CREATE INDEX attachments_drafts_idx ON attachments (uploaded_at) WHERE notice_id IS NULL;

ALTER TABLE attachments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON attachments USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON attachments TO dispute_app;

-- The dispatcher reads attachments across tenants when it sends; the sweep deletes drafts across tenants.
CREATE FUNCTION notice_attachments(nid bigint) RETURNS SETOF attachments
LANGUAGE sql SECURITY DEFINER SET search_path FROM CURRENT AS $$
    SELECT * FROM attachments WHERE notice_id = nid ORDER BY uploaded_at;
$$;

CREATE FUNCTION purge_draft_attachments(before timestamptz) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path FROM CURRENT AS $$
DECLARE n bigint;
BEGIN
    DELETE FROM attachments WHERE notice_id IS NULL AND uploaded_at < before;
    GET DIAGNOSTICS n = ROW_COUNT;
    RETURN n;
END;
$$;

REVOKE EXECUTE ON FUNCTION notice_attachments(bigint) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION purge_draft_attachments(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION notice_attachments(bigint) TO dispute_app;
GRANT EXECUTE ON FUNCTION purge_draft_attachments(timestamptz) TO dispute_app;
