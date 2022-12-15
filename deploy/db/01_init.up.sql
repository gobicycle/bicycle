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

CREATE TABLE IF NOT EXISTS payments.jetton_wallets
(
    subwallet_id     bigint not null, -- store uint32
    created_at       timestamptz not null default now(),
    user_id          text not null,
    currency         text not null,
    type             text not null,
    address          bytea not null unique
);

CREATE TABLE IF NOT EXISTS payments.internal_incomes
(
    lt                    bigint not null,
    utime                 timestamptz not null,
    deposit_address       bytea,
    hot_wallet_address    bytea,
    amount                numeric not null,
    memo                  uuid not null,
    unique (memo, lt)
);

CREATE TABLE IF NOT EXISTS payments.external_withdrawals
(
    msg_uuid         uuid,
    query_id         bigint,
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    processed_at     timestamptz,
    processed_lt     bigint,
    confirmed        bool not null default false,
    failed           bool not null default false,
    address          bytea,
    unique (msg_uuid, address)
);

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
    dest_address     bytea,
    comment          text,
    unique (user_id, user_query_id, is_internal)
);

CREATE TABLE IF NOT EXISTS payments.external_incomes
(
    lt               bigint not null,
    utime            timestamptz not null,
    deposit_address  bytea,
    payer_address    bytea,
    amount           numeric not null,
    comment          text not null
);

CREATE TABLE IF NOT EXISTS payments.block_data
(
        saved_at                timestamptz not null default now(),
        shard                   bigint not null,
        seqno                   bigint not null,
        gen_utime               timestamptz not null,
        root_hash               bytea not null,
        file_hash               bytea not null,
        unique (shard, seqno)
--      workchain int not null,
);

CREATE TABLE IF NOT EXISTS payments.internal_withdrawals
(
    failed           bool not null default false,
    since_lt         bigint not null,    -- amount for this LT
    finish_lt        bigint,             -- on deposit side
    finished_at      timestamptz,          -- on deposit side
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    amount           numeric not null default 0,
    from_address     bytea not null,
    memo             uuid not null unique,
    unique (from_address, since_lt, memo)
);

CREATE TABLE IF NOT EXISTS payments.service_withdrawal_requests
(
    memo uuid not null default gen_random_uuid() unique,
    created_at       timestamptz not null default now(),
    expired_at       timestamptz,
    processed        bool not null default false,
    ton_amount       numeric not null,
    jetton_amount    numeric not null,
    from_address     bytea not null,
    jetton_master    bytea
);

-- TODO: Add indexes

COMMIT;