-- A resend is a new notice chained to the one it repeats (docs/adr/0019): the original keeps its history, the
-- copy has its own delivery attempts and its own author.
ALTER TABLE notices ADD COLUMN resend_of bigint REFERENCES notices (id);
