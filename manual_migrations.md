# Manual migrations between versions

## v0.1.x -> v0.2.0
1. Apply [DB migration](/deploy/manual_migrations/0.1.x->0.2.0.sql)
2. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)

Note that this query creates a new column in the `external_incomes` DB table. All existing values for the payer address 
will be filled with a 0 workchain.

## v0.2.0 -> v0.3.0
1. Find the last master block scanned by the processor. To do this:
   * go to the [testnet explorer](https://test-explorer.toncoin.org) or [mainnet explorer](https://explorer.toncoin.org)
   * find the last scanned shard block in the current DB table `block_data` 
      ``` sql
       SELECT shard, seqno from payments.block_data ORDER BY seqno DESC LIMIT 1
      ```
   * select `Search block` in explorer and input: Workchain=0, shard (in hex format) and seqno for last shard bock from DB
   * click on the `masterchain block` field and go to the block description 
   * check `shard` section for your shard block
   * if the shard block seqno in the `shard` section is less than your shard block number - go to the next master blocks 
     using the `next block` until you find the right one
   * if you don't find the required shard block (seqno is missing) and/or the shard block seqno are out of order - 
     run the processor for a while and try again. (this is to avoid skipping scans of uncommitted shard blocks)
   * if you have found the required shard block in the section `shard` for the master block, then write down the 
     parameters of the master block: block (seqno), roothash, filehash, time
2. Edit [DB migration](/deploy/manual_migrations/0.2.0->0.3.0.sql) and fill last master block parameters
3. Apply [DB migration](/deploy/manual_migrations/0.2.0->0.3.0.sql)
4. Build new docker image and recreate container for `payment-processor` as described in `Service deploy` chapter in [Readme](/README.md)
