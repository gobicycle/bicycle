BEGIN;

INSERT INTO payments.external_incomes (
    lt,
    utime,
    deposit_address,
    payer_address,
    amount,
    comment
) VALUES
    (
        --      first payment to TON deposit A
        1,
        '2021-03-10 08:10:00 UTC',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab4', 'hex'),
        123,
        'test_comment_1'
    ),
    (
        --      second payment to TON deposit A
        2,
        '2021-03-10 08:11:00 UTC',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab5', 'hex'),
        123,
        'test_comment_2'
    ),
    (
        --      first payment to TON deposit B
        1,
        '2021-03-10 08:12:00 UTC',
        decode('01bb00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab6', 'hex'),
        123,
        'test_comment_3'
    ),
    (
        --      first payment to Jetton deposit C
        1,
        '2021-03-10 08:13:00 UTC',
        decode('01cc00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab7', 'hex'),
        123,
        'test_comment_4'
    );

INSERT INTO payments.internal_withdrawals (
    failed,
    since_lt,
    finish_lt,
    created_at,
    finished_at,
    expired_at,
    amount,
    from_address,
    memo
) VALUES
    (
        --      finished withdrawal from TON deposit A after first payment
        false,
        1,
        2,
        '2021-03-10 08:14:00 UTC',
        '2021-03-10 08:15:00 UTC',
        '2021-03-10 08:17:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7831'
    ),
    (
        --      failed withdrawal from TON deposit A after second payment
        true,
        2,
        NULL,
        '2021-03-10 08:16:00 UTC',
        NULL,
        '2021-03-10 08:19:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7832'
    ),
    (
        --      not finished withdrawal from TON deposit B after payment
        false,
        1,
        NULL,
        '2021-03-10 08:14:00 UTC',
        NULL,
        '2021-03-10 08:17:00 UTC',
        100,
        decode('01bb00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7833'
    );

INSERT INTO payments.ton_wallets (
    subwallet_id,
    created_at,
    user_id,
    type,
    address
) VALUES
    (
        --      TON hot wallet
        1,
        '2021-03-10 08:00:00 UTC',
        '',
        'ton_hot',
        decode('00aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    ),
    (
        --      TON deposit A
        2,
        '2021-03-10 08:00:00 UTC',
        'test_user',
        'ton_deposit',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    ),
    (
        --      TON deposit B
        3,
        '2021-03-10 08:00:00 UTC',
        'test_user',
        'ton_deposit',
        decode('01bb00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    ),
    (
        --      Jetton deposit C
        4,
        '2021-03-10 08:00:00 UTC',
        'test_user',
        'owner',
        decode('01cc00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    );

COMMIT;