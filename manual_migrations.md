# Manual migrations between versions

## v0.1.x -> v0.2.0
1. Apply [DB migration](/deploy/manual_migrations/0.1.x->0.2.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new column in the `external_incomes` DB table. All existing values for the payer address 
will be filled with a 0 workchain.