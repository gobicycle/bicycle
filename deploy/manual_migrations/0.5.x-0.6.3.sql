CREATE INDEX IF NOT EXISTS internal_withdrawals_failed_from_address_index
    ON payments.internal_withdrawals (failed, from_address);
