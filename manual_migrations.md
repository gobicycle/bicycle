# Manual migrations between versions

## v0.1.x -> v0.2.0
1. Apply [DB migration](/deploy/manual_migrations/0.1.x-0.2.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new column in the `external_incomes` DB table. All existing values for the payer address 
will be filled with a 0 workchain.

## v0.4.x -> v0.5.0
1. Apply [DB migration](/deploy/manual_migrations/0.4.x-0.5.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new nullable column in the `external_withdrawals` and `external_incomes` DB tables and `binary_comment` column in `withdrawal_requests` table.

## 0.10.x -> 0.11.0 
(**Optional**. Recommended if you are using proofs checking (PROOF_CHECK_ENABLED=true))
1. Apply [DB migration](/deploy/manual_migrations/0.10.x-0.11.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new table `payments.last_proven_block`. 
It will be filled automatically based on config file (NETWORK_CONFIG_URL) and archive node, but you can skip requirements for an archive node and fill it manually:

Go to https://tonviewer.com/last, take **seqno**, **root_hash** and **file_hash** and put them into the request below.
For example 69722697, 4b7e27d3cc60bf16c71be02de8eeb1b874c91de649e62f3ad088d5eb7a6a8d30, 418969d0941d9463a158bffe273852e34b35134a38def02690ecc895b510baa8
```sql
INSERT INTO payments.last_proven_block ( slug,  workchain,  shard, seqno, root_hash,  file_hash) 
VALUES ('head', -1, -9223372036854775808, 69722697, E'\\x4b7e27d3cc60bf16c71be02de8eeb1b874c91de649e62f3ad088d5eb7a6a8d30', E'\\x418969d0941d9463a158bffe273852e34b35134a38def02690ecc895b510baa8')
```