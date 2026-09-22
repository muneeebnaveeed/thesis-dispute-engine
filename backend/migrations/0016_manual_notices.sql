-- Analyst-initiated communications (docs/adr/0019): the same notices table, four more kinds, and who sent them.
ALTER TABLE notices DROP CONSTRAINT notices_kind_check;
ALTER TABLE notices ADD CONSTRAINT notices_kind_check CHECK (kind IN (
    'ACKNOWLEDGEMENT', 'QUESTIONNAIRE', 'PROVISIONAL_CREDIT', 'REFUND', 'REVERSAL', 'RESOLUTION',
    'REQUEST_FOR_INFORMATION', 'STATUS_UPDATE', 'DOCUMENTS_RECEIVED', 'CUSTOM'));
ALTER TABLE notices ADD COLUMN actor text;   -- the analyst who composed it; NULL for the engine's own notices
-- A manual notice is not tied to one event, so several may follow the same event.
ALTER TABLE notices DROP CONSTRAINT notices_dispute_id_seq_kind_channel_key;
CREATE UNIQUE INDEX notices_automatic_once_idx ON notices (dispute_id, seq, kind, channel) WHERE actor IS NULL;
