BEGIN;

INSERT INTO payments.external_withdrawals (
    msg_uuid,
    query_id,
    created_at,
    expired_at,
    processed_at,
    processed_lt,
    confirmed,
    failed,
    address
) VALUES
    (
        --      expired and not marked as expired 1st
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        1,
        '2000-01-01 08:00:00 UTC',
        '2000-01-01 08:03:00 UTC',
        NULL,
        NULL,
        false,
        false,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab1', 'hex')
    ),
    (
        --      expired and not marked as expired 2nd
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        2,
        '2000-01-01 08:00:00 UTC',
        '2000-01-01 08:04:00 UTC',
        NULL,
        NULL,
        false,
        false,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    ),
    (
        --      expired and already marked as expired
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        3,
        '2000-01-01 08:00:00 UTC',
        '2000-01-01 08:05:00 UTC',
        NULL,
        NULL,
        false,
        true,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab3', 'hex')
    ),
    (
        --      not expired
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        4,
        '2000-01-01 08:00:00 UTC',
        '3000-01-01 08:00:00 UTC',
        NULL,
        NULL,
        false,
        false,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab4', 'hex')
    ),
    (
        --      not expired but failed
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        5,
        '2000-01-01 08:00:00 UTC',
        '3000-01-01 08:00:00 UTC',
        NULL,
        NULL,
        false,
        true,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab5', 'hex')
    ),
    (
        --      not expired and already processed
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        6,
        '2000-01-01 08:00:00 UTC',
        '3000-01-01 08:00:00 UTC',
        '2000-01-01 08:10:00 UTC',
        1,
        false,
        false,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab6', 'hex')
    ),
    (
        --      expired and already processed
        'c2d29867-3d0b-d497-9191-18a9d8ee7831',
        7,
        '2000-01-01 08:00:00 UTC',
        '2000-01-01 08:03:00 UTC',
        '2000-01-01 08:01:00 UTC',
        2,
        false,
        false,
        decode('007424ee4767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab7', 'hex')
    );

INSERT INTO payments.internal_withdrawals (
    failed,
    since_lt,
    sending_lt,
    finish_lt,
    created_at,
    finished_at,
    expired_at,
    amount,
    from_address,
    memo
) VALUES
    (
        --      expired and not marked as expired
        false,
        1,
        NULL,
        NULL,
        '2020-03-10 08:00:00 UTC',
        NULL,
        '2020-03-10 08:03:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7831'
    ),
    (
        --      expired and already marked as expired
        true,
        2,
        NULL,
        NULL,
        '2020-03-10 08:00:00 UTC',
        NULL,
        '2020-03-10 08:03:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7832'
    ),
    (
        --      not expired
        false,
        3,
        4,
        NULL,
        '2020-03-10 08:00:00 UTC',
        NULL,
        '3020-03-10 08:00:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7833'
    ),
    (
        --      not expired but failed
        true,
        4,
        5,
        NULL,
        '2020-03-10 08:00:00 UTC',
        NULL,
        '3020-03-10 08:00:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7834'
    ),
    (
        --      not expired but already processed
        false,
        5,
        6,
        6,
        '2020-03-10 08:00:00 UTC',
        '2020-03-10 08:01:00 UTC',
        '3020-03-10 08:00:00 UTC',
        100,
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7835'
    ),
    (
        --      expired and already processed
        false,
        6,
        7,
        7,
        '2020-03-10 08:00:00 UTC',
        '2020-03-10 08:01:00 UTC',
        '2020-03-10 08:03:00 UTC',
        100,
        decode('01bb00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex'),
        'c2d29867-3d0b-d497-9191-18a9d8ee7836'
    );

INSERT INTO payments.withdrawal_requests (
    query_id,
    bounceable,
    processing,
    processed,
    is_internal,
    amount,
    user_id,
    user_query_id,
    currency,
    comment,
    dest_address
) VALUES
    (
        --      request with query_id 1 for external_withdrawals
        1, false, true, false, false,
        100, 'test_user', '1', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab1', 'hex')
    ),
    (
        --      request with query_id 2 for external_withdrawals
        2, false, true, false, false,
        100, 'test_user', '2', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab2', 'hex')
    ),
    (
        --      request with query_id 3 for external_withdrawals
        3, false, false, false, false,
        100, 'test_user', '3', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab3', 'hex')
    ),
    (
        --      request with query_id 4 for external_withdrawals
        4, false, true, false, false,
        100, 'test_user', '4', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab4', 'hex')
    ),
    (
        --      request with query_id 5 for external_withdrawals
        5, false, false, false, false,
        100, 'test_user', '5', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab5', 'hex')
    ),
    (
        --      request with query_id 6 for external_withdrawals
        6, false, true, true, false,
        100, 'test_user', '6', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab6', 'hex')
    ),
    (
        --      request with query_id 7 for external_withdrawals
        7, false, true, true, false,
        100, 'test_user', '7', 'TON', 'test_comment',
        decode('01aa00004767fbcf859609200910269446980f4d27bd8f4e3faa6e4d74792ab7', 'hex')
    );

COMMIT;