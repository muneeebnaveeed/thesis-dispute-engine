-- Fraud scoring (docs/adr/0018): an explainable, rule-based assessment recorded when a dispute opens and again
-- when the questionnaire arrives. Every assessment is kept, so the analyst sees how the picture changed.
ALTER TABLE transactions ADD COLUMN mcc char(4);   -- merchant category code, when the rail supplies one

CREATE TABLE risk_assessments (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id    uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id   uuid NOT NULL REFERENCES disputes (id),
    seq          int  NOT NULL CHECK (seq > 0),          -- the dispute event that prompted it
    score        int  NOT NULL CHECK (score >= 0),
    tier         text NOT NULL CHECK (tier IN ('LOW', 'MEDIUM', 'HIGH')),
    signals      jsonb NOT NULL,
    assessed_at  timestamptz NOT NULL,
    UNIQUE (dispute_id, seq)
);

CREATE INDEX risk_assessments_dispute_idx ON risk_assessments (dispute_id, id DESC);

ALTER TABLE risk_assessments ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON risk_assessments USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT ON risk_assessments TO dispute_app;

-- The list can be filtered to what needs a human: the latest tier per dispute.
CREATE INDEX disputes_account_opened_idx ON disputes (account_id, opened_at);
