BEGIN;

ALTER TABLE payments.block_data
ADD COLUMN IF NOT EXISTS is_master bool;

UPDATE payments.block_data SET is_master = false WHERE is_master IS NULL;  -- all existing blocks will be marked as shard blocks

ALTER TABLE payments.block_data
    ALTER COLUMN is_master SET NOT NULL;

ALTER TABLE payments.block_data
    DROP CONSTRAINT IF EXISTS block_data_shard_seqno_key;

ALTER TABLE payments.block_data
    ADD CONSTRAINT block_data_shard_seqno_is_master_key UNIQUE (shard, seqno, is_master);

INSERT INTO payments.block_data(saved_at, shard, seqno, gen_utime, root_hash, file_hash, is_master)
VALUES (
           now(),
           -9223372036854775808, -- only one shard for master. Do not change!
           29245828, -- set last scanned master block seqno here
           to_timestamp(1683091782), -- set last scanned master block time here
           decode('DA8F022821787478B520845DB2618F9CDAD772DCD26E672D4D422F0537EB8061','hex'), -- set last scanned master block roothash here
           decode('19ACCBCBB922D9E447CD79161464955A337D4EAD0F3F098FCE1A8DF0C2CD20B2','hex'), -- set last scanned master block filehash here
           true
       );

COMMIT;