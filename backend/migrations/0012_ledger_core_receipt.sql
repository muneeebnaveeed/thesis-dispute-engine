-- What the tenant's banking core answered for a posting that moved the customer's money (docs/adr/0015).
-- Internal postings (suspense against recovery or loss) carry nothing here.
ALTER TABLE ledger_entries ADD COLUMN core_rrn           text;
ALTER TABLE ledger_entries ADD COLUMN core_response_code text;
ALTER TABLE ledger_entries ADD COLUMN core_latency_ms    int;
