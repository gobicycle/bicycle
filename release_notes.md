# v0.5.0 release notes

1. Add `tx_hash` field, `sort_order` parameter to `/v1/deposit/history{?user_id,currency,limit,offset}` method
2. Add `tx_hash`, `user_id`, `query_id` fields to `/v1/withdrawal/status{?id}` method response
3. Add `failed` status to `/v1/withdrawal/status{?id}` method and withdrawal processor do not retry failed withdrawals (retry only for lost external messages)
4. New method `/v1/deposit/income{?tx_hash}` to find income by `tx_hash`
5. New method `/v1/balance{?currency,address}` to get account balance from blockchain and pending and processing amounts for hot wallet
6. Push notifications (webhook or rabbitmq) after save to db (see new notification logic in `README.md` and `technical_notes.md`)
7. Postgres docker volume permanent by default
8. Audit log error counters with prometheus metrics
9. Update dependencies
10. Bugfixes
11. Improve stability
12. New method `/v1/resolve{?domain}` to resolve wallet DNS record
13. Add `last_block_gen_utime` field to `/v1/system/sync` method to get unix time of the last scanned block
14. Add `FORWARD_TON_AMOUNT` env variable to customize `forward_ton_amount` for Jetton withdrawals
15. Binary comment support
