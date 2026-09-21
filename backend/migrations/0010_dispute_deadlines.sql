-- Regulatory clocks (docs/adr/0013). One row per clock a regime starts on a dispute; the due time is computed once
-- in the tenant's calendar and the clock is settled (met or voided) by the transition that satisfies it. Status is
-- derived at read time from due_at, met_at and voided_at, so a breach needs no job to record it.
CREATE TABLE dispute_deadlines (
    tenant_id   uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id  uuid NOT NULL REFERENCES disputes (id),
    kind        text NOT NULL CHECK (kind IN ('REFUND', 'ACKNOWLEDGE', 'RESOLUTION')),
    cycle       int  NOT NULL DEFAULT 0 CHECK (cycle >= 0),
    started_at  timestamptz NOT NULL,
    due_at      timestamptz NOT NULL,
    met_at      timestamptz,
    voided_at   timestamptz,
    basis       text NOT NULL,
    PRIMARY KEY (dispute_id, kind, cycle),
    CHECK (met_at IS NULL OR voided_at IS NULL)
);

-- The overdue list and the gauge both ask "open clocks past due for this tenant", newest due first.
CREATE INDEX dispute_deadlines_open_idx ON dispute_deadlines (tenant_id, due_at) WHERE met_at IS NULL AND voided_at IS NULL;

ALTER TABLE dispute_deadlines ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON dispute_deadlines USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON dispute_deadlines TO dispute_app;

-- Existing disputes predate the clocks. They keep none: a clock invented after the fact would be due in the past and
-- report a breach nobody could have acted on. New disputes start theirs at creation.

-- Cross-tenant count for the dispute.deadlines_overdue gauge; runs with the owner's privileges like disputes_by_state.
CREATE VIEW deadlines_overdue AS
    SELECT d.tenant_id, s.regime, d.kind, count(*)::bigint AS n
    FROM dispute_deadlines d
    JOIN disputes s ON s.id = d.dispute_id
    WHERE d.met_at IS NULL AND d.voided_at IS NULL AND d.due_at < now()
    GROUP BY d.tenant_id, s.regime, d.kind;
GRANT SELECT ON deadlines_overdue TO dispute_app;
