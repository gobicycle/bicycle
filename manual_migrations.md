# Manual migrations between versions

## v0.1.x -> v0.2.0
1. Apply [DB migration](/deploy/manual_migrations/0.1.x-0.2.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new column in the `external_incomes` DB table. All existing values for the payer address 
will be filled with a 0 workchain.

## v0.4.x -> v0.5.0
1. Apply [DB migration](/deploy/manual_migrations/0.4.x-0.5.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new nullable column in the `external_withdrawals` and `external_incomes` DB tables.