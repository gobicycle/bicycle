CREATE TABLE IF NOT EXISTS payments.last_proven_block (
       slug text PRIMARY KEY,
           workchain               bigint not null,
           shard                   bigint not null,
           seqno                   bigint not null,
           root_hash               bytea not null,
           file_hash               bytea not null
);