BEGIN;

CREATE SCHEMA IF NOT EXISTS payments;

CREATE TABLE IF NOT EXISTS payments.ton_wallets
(
    subwallet_id     bigint not null unique, -- store uint32
    created_at       timestamptz not null default now(),
    user_id          text not null,
    type             text not null,
    address          bytea not null
);

CREATE INDEX IF NOT EXISTS ton_wallets_address_index
    ON payments.ton_wallets (address);

CREATE INDEX IF NOT EXISTS ton_wallets_type_index
    ON payments.ton_wallets (type);

CREATE INDEX IF NOT EXISTS ton_wallets_user_id_index
    ON payments.ton_wallets (user_id);

CREATE TABLE IF NOT EXISTS payments.jetton_wallets
(
    subwallet_id     bigint not null, -- store uint32
    created_at       timestamptz not null default now(),
    user_id          text not null,
    currency         text not null,
    type             text not null,
    address          bytea not null unique
);

CREATE INDEX IF NOT EXISTS jetton_wallets_subwallet_id_index
    ON payments.jetton_wallets (subwallet_id);

CREATE TABLE IF NOT EXISTS payments.internal_incomes
(
    lt                    bigint not null,
    utime                 timestamptz not null,
    deposit_address       bytea not null,
    amount                numeric not null,
    memo                  uuid not null,
    unique (memo, lt)
);

CREATE INDEX IF NOT EXISTS internal_incomes_deposit_address_index
    ON payments.internal_incomes (deposit_address);

CREATE TABLE IF NOT EXISTS payments.external_withdrawals
(
    msg_uuid         uuid not null,
    query_id         bigint not null,
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    processed_at     timestamptz,
    processed_lt     bigint,
    confirmed        bool not null default false,
    failed           bool not null default false,
    address          bytea not null,
    tx_hash          bytea,
    unique (msg_uuid, address)
);

CREATE INDEX IF NOT EXISTS external_withdrawals_expired_at_index
    ON payments.external_withdrawals (expired_at);

CREATE INDEX IF NOT EXISTS external_withdrawals_msg_uuid_index
    ON payments.external_withdrawals (msg_uuid);

CREATE INDEX IF NOT EXISTS external_withdrawals_address_index
    ON payments.external_withdrawals (address);

CREATE INDEX IF NOT EXISTS external_withdrawals_query_id_index
    ON payments.external_withdrawals (query_id);

CREATE TABLE IF NOT EXISTS payments.withdrawal_requests
(
    query_id         bigserial
                     constraint withdrawal_requests_pk
                     primary key,
    bounceable       bool  not null,
    processing       bool not null default false,
    processed        bool not null default false,
    is_internal      bool default false,
    amount           numeric not null,
    user_id          text not null,
    user_query_id    text not null,
    currency         text not null,
    dest_address     bytea not null,
    comment          text,
    binary_comment   text,
    unique (user_id, user_query_id, is_internal)
);

CREATE INDEX IF NOT EXISTS withdrawal_requests_user_id_index
    ON payments.withdrawal_requests (user_id);

CREATE INDEX IF NOT EXISTS withdrawal_requests_user_query_id_index
    ON payments.withdrawal_requests (user_query_id);

CREATE INDEX IF NOT EXISTS withdrawal_requests_dest_address_index
    ON payments.withdrawal_requests (dest_address);

CREATE TABLE IF NOT EXISTS payments.external_incomes
(
    lt               bigint not null,
    utime            timestamptz not null,
    payer_workchain  integer,
    deposit_address  bytea not null,
    payer_address    bytea,
    amount           numeric not null,
    comment          text not null,
    tx_hash          bytea
);

CREATE INDEX IF NOT EXISTS external_incomes_deposit_address_index
    ON payments.external_incomes (deposit_address);

CREATE TABLE IF NOT EXISTS payments.block_data
(
        saved_at                timestamptz not null default now(),
        shard                   bigint not null,
        seqno                   bigint not null,
        gen_utime               timestamptz not null,
        root_hash               bytea not null,
        file_hash               bytea not null,
        unique (shard, seqno)
);

CREATE INDEX IF NOT EXISTS block_data_seqno_index
    ON payments.block_data (seqno);

CREATE TABLE IF NOT EXISTS payments.internal_withdrawals
(
    failed           bool not null default false,
    since_lt         bigint not null,    -- amount for this LT
    sending_lt       bigint,             -- on wallet side
    finish_lt        bigint,             -- on deposit side
    finished_at      timestamptz,        -- on deposit side
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    amount           numeric not null default 0,
    from_address     bytea not null,
    memo             uuid not null unique,
    unique (from_address, since_lt, memo)
);

CREATE INDEX IF NOT EXISTS internal_withdrawals_from_address_index
    ON payments.internal_withdrawals (from_address);

CREATE INDEX IF NOT EXISTS internal_withdrawals_since_lt_index
    ON payments.internal_withdrawals (since_lt);

CREATE INDEX IF NOT EXISTS internal_withdrawals_expired_at_index
    ON payments.internal_withdrawals (expired_at);

CREATE TABLE IF NOT EXISTS payments.service_withdrawal_requests
(
    memo uuid not null default gen_random_uuid() unique,
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    filled           bool not null default false,
    processed        bool not null default false,
    ton_amount       numeric not null default 0,
    jetton_amount    numeric not null default 0,
    from_address     bytea not null,
    jetton_master    bytea
);

COMMIT;