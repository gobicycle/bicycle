BEGIN;

ALTER TABLE payments.external_incomes
ADD COLUMN IF NOT EXISTS payer_workchain integer;

UPDATE payments.external_incomes SET payer_workchain = 0 WHERE payer_address IS NOT NULL AND payer_workchain IS NULL;  -- all existing addresses will be marked as 0 workchain

COMMIT;