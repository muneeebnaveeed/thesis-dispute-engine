-- The W3C trace context of the request that queued a notice, so the dispatcher's delivery span can link back to it
-- and one trace shows the click, the transition and the mail. Text, not parsed: the database never reads it.
ALTER TABLE notices ADD COLUMN trace_context text;

CREATE OR REPLACE FUNCTION claim_notices(batch int) RETURNS SETOF notices
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

-- How many emails wait in the outbox, for the backlog gauge; owner-defined like the claim, because the dispatcher
-- has no tenant.
CREATE FUNCTION outbox_backlog() RETURNS bigint
LANGUAGE sql SECURITY DEFINER SET search_path FROM CURRENT STABLE AS $$
    SELECT count(*) FROM notices WHERE sent_at IS NULL AND channel = 'EMAIL';
$$;
REVOKE EXECUTE ON FUNCTION outbox_backlog() FROM PUBLIC;
GRANT EXECUTE ON FUNCTION outbox_backlog() TO dispute_app;
