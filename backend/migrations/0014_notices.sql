-- Communications (docs/adr/0017). Every notice the engine owes the customer is a row written in the same
-- transaction as the transition that caused it; email rows are an outbox a dispatcher drains, letter rows are
-- ready to print the moment they exist. The customer's addresses live on the account.
ALTER TABLE accounts ADD COLUMN email          text;
ALTER TABLE accounts ADD COLUMN postal_address text;

CREATE TABLE notices (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id        uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id       uuid NOT NULL REFERENCES disputes (id),
    seq              int  NOT NULL CHECK (seq > 0),                 -- the dispute event that caused it
    kind             text NOT NULL CHECK (kind IN ('ACKNOWLEDGEMENT', 'QUESTIONNAIRE', 'PROVISIONAL_CREDIT', 'REFUND', 'REVERSAL', 'RESOLUTION')),
    channel          text NOT NULL CHECK (channel IN ('EMAIL', 'LETTER')),
    recipient        text NOT NULL,                                 -- email address or postal address
    subject          text NOT NULL,
    document         jsonb NOT NULL,                                -- the composed paragraphs (notice.Document)
    created_at       timestamptz NOT NULL,
    sent_at          timestamptz,
    attempts         int  NOT NULL DEFAULT 0,
    next_attempt_at  timestamptz NOT NULL DEFAULT now(),
    last_error       text,
    UNIQUE (dispute_id, seq, kind, channel)
);

CREATE INDEX notices_dispute_idx ON notices (dispute_id, id);
CREATE INDEX notices_outbox_idx  ON notices (next_attempt_at) WHERE sent_at IS NULL AND channel = 'EMAIL';

ALTER TABLE notices ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON notices USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT ON notices TO dispute_app;

-- The dispatcher serves every tenant and has no tenant of its own, so it reaches the outbox through owner-defined
-- functions, like the purge. Claiming marks the attempt and pushes the next one out, so a crash mid-send retries
-- later rather than never; SKIP LOCKED lets replicas share the work.
CREATE FUNCTION claim_notices(batch int) RETURNS SETOF notices
LANGUAGE plpgsql SECURITY DEFINER SET search_path FROM CURRENT AS $$
BEGIN
    RETURN QUERY
    UPDATE notices n
    SET attempts = n.attempts + 1,
        next_attempt_at = now() + make_interval(secs => least(3600, 30 * power(2, n.attempts)))
    WHERE n.id IN (
        SELECT id FROM notices
        WHERE sent_at IS NULL AND channel = 'EMAIL' AND next_attempt_at <= now()
        ORDER BY next_attempt_at
        LIMIT batch
        FOR UPDATE SKIP LOCKED)
    RETURNING n.*;
END;
$$;

CREATE FUNCTION finish_notice(notice_id bigint, failure text) RETURNS void
LANGUAGE sql SECURITY DEFINER SET search_path FROM CURRENT AS $$
    UPDATE notices
    SET sent_at = CASE WHEN failure IS NULL THEN now() ELSE sent_at END,
        last_error = failure
    WHERE id = notice_id;
$$;

REVOKE EXECUTE ON FUNCTION claim_notices(int) FROM PUBLIC;
REVOKE EXECUTE ON FUNCTION finish_notice(bigint, text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION claim_notices(int) TO dispute_app;
GRANT EXECUTE ON FUNCTION finish_notice(bigint, text) TO dispute_app;
