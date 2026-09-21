-- The dispute ledger (docs/adr/0014): every movement the engine instructs on a dispute, double-entry between the
-- customer's account, the dispute suspense account and the recovery and loss accounts. Rows are written by the
-- transition that causes them and never change; a correction is a new posting.
CREATE TABLE ledger_entries (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       uuid NOT NULL REFERENCES tenants (id) DEFAULT current_tenant_id(),
    dispute_id      uuid NOT NULL REFERENCES disputes (id),
    seq             int  NOT NULL CHECK (seq > 0),                 -- the dispute event that caused it
    kind            text NOT NULL CHECK (kind IN ('PROVISIONAL_CREDIT', 'FAST_REFUND', 'NQA_REFUND',
                                                  'PROVISIONAL_CREDIT_REVERSAL', 'RECOVERY', 'WRITE_OFF')),
    debit_account   text NOT NULL CHECK (debit_account  IN ('CUSTOMER', 'SUSPENSE', 'RECOVERY', 'LOSS')),
    credit_account  text NOT NULL CHECK (credit_account IN ('CUSTOMER', 'SUSPENSE', 'RECOVERY', 'LOSS')),
    amount          numeric(19, 4) NOT NULL CHECK (amount > 0),
    currency        char(3) NOT NULL,
    reference       text NOT NULL,                                 -- what the banking core is told; unique per posting
    posted_at       timestamptz NOT NULL,
    CHECK (debit_account <> credit_account),
    UNIQUE (dispute_id, seq, kind),
    UNIQUE (tenant_id, reference)
);

CREATE INDEX ledger_entries_dispute_idx ON ledger_entries (dispute_id, id);

-- Same guard as the event log, named for whichever table it protects.
CREATE FUNCTION append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION '% is append-only (% refused)', TG_TABLE_NAME, TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;

CREATE TRIGGER ledger_entries_no_update
    BEFORE UPDATE OR DELETE ON ledger_entries
    FOR EACH ROW EXECUTE FUNCTION append_only();

CREATE TRIGGER ledger_entries_no_truncate
    BEFORE TRUNCATE ON ledger_entries
    FOR EACH STATEMENT EXECUTE FUNCTION append_only();

ALTER TABLE ledger_entries ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON ledger_entries USING (tenant_id = current_tenant_id()) WITH CHECK (tenant_id = current_tenant_id());
GRANT SELECT, INSERT ON ledger_entries TO dispute_app;

-- Cross-tenant suspense per regime for the dispute.suspense gauge; owner privileges like the other views.
CREATE VIEW suspense_by_regime AS
    SELECT l.tenant_id, d.regime, l.currency,
           sum(CASE WHEN l.debit_account = 'SUSPENSE' THEN l.amount ELSE 0 END)
         - sum(CASE WHEN l.credit_account = 'SUSPENSE' THEN l.amount ELSE 0 END) AS balance
    FROM ledger_entries l
    JOIN disputes d ON d.id = l.dispute_id
    GROUP BY l.tenant_id, d.regime, l.currency;
GRANT SELECT ON suspense_by_regime TO dispute_app;
