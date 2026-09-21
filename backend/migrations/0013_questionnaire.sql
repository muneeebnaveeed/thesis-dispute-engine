-- Why the customer disputes, and the questionnaire that follows from it (docs/adr/0016). The questions are
-- snapshotted when sent so answers always match what was asked; answers arrive with RECEIVE_QUESTIONNAIRE.
ALTER TABLE disputes ADD COLUMN reason text NOT NULL DEFAULT 'UNAUTHORISED'
    CHECK (reason IN ('UNAUTHORISED', 'NOT_RECEIVED', 'DUPLICATE', 'AMOUNT_DIFFERS'));

CREATE TABLE questionnaires (
    tenant_id    uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id   uuid PRIMARY KEY REFERENCES disputes (id),
    reason       text NOT NULL,
    questions    jsonb NOT NULL,
    answers      jsonb,
    sent_at      timestamptz NOT NULL,
    received_at  timestamptz,
    CHECK ((answers IS NULL) = (received_at IS NULL))
);

ALTER TABLE questionnaires ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON questionnaires USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT, UPDATE ON questionnaires TO dispute_app;
