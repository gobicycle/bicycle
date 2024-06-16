BEGIN;

ALTER TABLE payments.external_withdrawals
    ADD COLUMN IF NOT EXISTS tx_hash bytea;

ALTER TABLE payments.withdrawal_requests
    ADD COLUMN IF NOT EXISTS binary_comment text default '';

ALTER TABLE payments.external_incomes
    ADD COLUMN IF NOT EXISTS tx_hash bytea;

COMMIT;