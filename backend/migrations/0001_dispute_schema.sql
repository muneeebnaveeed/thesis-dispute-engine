CREATE TABLE accounts (
    id          uuid PRIMARY KEY,
    holder_name text NOT NULL,
    currency    char(3) NOT NULL,
    opened_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE transactions (
    id           uuid PRIMARY KEY,
    account_id   uuid NOT NULL REFERENCES accounts (id),
    rail         text NOT NULL CHECK (rail IN ('CARD', 'CREDIT_CARD', 'SEPA_DD')),
    amount       numeric(19, 4) NOT NULL CHECK (amount > 0),
    currency     char(3) NOT NULL,
    merchant     text NOT NULL,
    occurred_at  timestamptz NOT NULL
);

CREATE INDEX transactions_account_id_idx ON transactions (account_id, occurred_at DESC);

CREATE TABLE disputes (
    id               uuid PRIMARY KEY,
    regime           text NOT NULL CHECK (regime IN ('EU_SEPA_DIRECT_DEBIT', 'EU_PSD2_CARD', 'US_REG_E', 'US_REG_Z')),
    state            text NOT NULL,
    appeals          int NOT NULL DEFAULT 0 CHECK (appeals >= 0),
    version          bigint NOT NULL DEFAULT 0 CHECK (version >= 0),
    transaction_id   uuid NOT NULL REFERENCES transactions (id),
    account_id       uuid NOT NULL REFERENCES accounts (id),
    disputed_amount  numeric(19, 4) NOT NULL CHECK (disputed_amount > 0),
    currency         char(3) NOT NULL,
    opened_at        timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX disputes_account_id_idx ON disputes (account_id, opened_at DESC);
CREATE INDEX disputes_state_idx ON disputes (state) WHERE state <> 'CLOSED';

CREATE TABLE dispute_events (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    dispute_id       uuid NOT NULL REFERENCES disputes (id),
    seq              int NOT NULL CHECK (seq > 0),
    event            text NOT NULL,
    from_state       text NOT NULL,
    to_state         text NOT NULL,
    actor            text NOT NULL,
    payload          jsonb NOT NULL DEFAULT '{}'::jsonb,
    idempotency_key  text,
    trace_id         text,
    occurred_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (dispute_id, seq)
);

-- The log is the audit trail; mutation is refused at the database, not by convention.
CREATE FUNCTION dispute_events_append_only() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'dispute_events is append-only (% refused)', TG_OP
        USING ERRCODE = 'integrity_constraint_violation';
END;
$$;

CREATE TRIGGER dispute_events_no_update
    BEFORE UPDATE OR DELETE ON dispute_events
    FOR EACH ROW EXECUTE FUNCTION dispute_events_append_only();

CREATE TRIGGER dispute_events_no_truncate
    BEFORE TRUNCATE ON dispute_events
    FOR EACH STATEMENT EXECUTE FUNCTION dispute_events_append_only();

CREATE TABLE idempotency_keys (
    scope         text NOT NULL,
    key           text NOT NULL,
    request_hash  bytea NOT NULL,
    status_code   int NOT NULL,
    response      jsonb NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (scope, key)
);

CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);
