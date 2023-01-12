BEGIN;

DROP TABLE IF EXISTS payments.ton_wallets;
DROP TABLE IF EXISTS payments.jetton_wallets;
DROP TABLE IF EXISTS payments.internal_incomes;
DROP TABLE IF EXISTS payments.external_withdrawals;
DROP TABLE IF EXISTS payments.withdrawal_requests;
DROP TABLE IF EXISTS payments.external_incomes;
DROP TABLE IF EXISTS payments.block_data;
DROP TABLE IF EXISTS payments.internal_withdrawals;
DROP TABLE IF EXISTS payments.service_withdrawal_requests;

DROP SCHEMA IF EXISTS payments;

COMMIT;