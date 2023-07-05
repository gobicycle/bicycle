## Manual testing plan for v0.4.0
Template:
-[x] Checked
- TEST    : test description
- RESULT  : expected result
- COMMENT : some comment 

### Initialization

1. -[x] Checked
- TEST    : Run with not deployed (and zero balance) hot wallet (new seed phrase)
- RESULT  : There must be an insufficient balance error
- COMMENT : 

2. -[x] Checked
- TEST    : Run with uninit hot wallet with balance > minimum balance
- RESULT  : Hot wallet must be initialized at first start of service
- COMMENT :

3. -[x] Checked
- TEST    : Run with new seed phrase when hot wallet already exist in DB
- RESULT  : There must be an incorrect seed phrase error
- COMMENT :

4. -[x] Checked
- TEST    : Run service with empty DB and stop after few minutes. Check time of first and last block 
            in `block_data` table
- RESULT  : Time `saved_at` and `gen_utime` must correlate with system time
- COMMENT :

5. -[x] Checked
- TEST    : Run with nonexist hot jetton wallet and receive external jetton transfer at jetton deposit 
            (> minimal withdrawal amount)
- RESULT  : Jetton hot wallet must be initialized by Jetton withdrawal from deposit, 
            if jetton deposit successfully initialized (it depends on transfer sender)
- COMMENT :

6. -[x] Checked
- TEST    : Run with testnet cold wallet address at mainnet (`IS_TESTNET=false`)
- RESULT  : There must be "Can not use testnet cold wallet address for mainnet" error
- COMMENT :

7. -[x] Checked
- TEST    : Run service with empty `JETTONS` env variable
- RESULT  : Service must start and process TONs
- COMMENT :

8. -[x] Checked
- TEST    : Run service with `JETTONS` env variable with different currencies and same master contract address.
            Like `TGR:ABC...,FNZ:ABC...`.
- RESULT  : Service must stop. Must be address duplication error message in audit log.
- COMMENT :

9. -[x] Checked
- TEST    : Run service with one `JETTONS` env variable, then rename currency for one of Jetton and restart.
            Like `TGR:ABC...,FNZ:CDE...` -> `SCALE:ABC...,FNZ:CDE...`.
- RESULT  : Service must stop. Must be address duplication error message in audit log.
- COMMENT :

10. -[x] Checked
- TEST    : Start service with uninitialized cold wallet and bounceable address for cold wallet.
- RESULT  : Service must stop. Must be invalid address format error message in log.
- COMMENT :

### API

1. -[x] Checked
- TEST    : Use `/v1/address/new` method (few for TONs and few for Jettons for different users). 
            Check new addresses in DB
- RESULT  : You must receive different addresses in user-friendly format with `bounce = false` flag and 
            testnet flag correlated with `IS_TESTNET` env var and raw in DB. For Jetton deposits it must be an
            owner address.
- COMMENT :

2. -[x] Checked
- TEST    : Use `/v1/address/all{?user_id}` method and compare with addresses created at 1. And check it by DB
- RESULT  : All addresses must be received and equal to those created earlier
- COMMENT :

3. -[x] Checked
- TEST    : Check `/v1/income{?user_id}` for new empty deposits
- RESULT  : Income must be zero. The addresses must match the addresses obtained by method `/v1/address/all{?user_id}`.
- COMMENT :

4. -[x] Checked
- TEST    : Make some payments at deposits and check it by `/v1/income{?user_id}` method 
            with different `DEPOSIT_SIDE_BALANCE` env var
- RESULT  : Income must correlate with payments sum
- COMMENT :

5. -[x] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for TONs and Jettons with amount > hot wallet balance
            and check it by `/v1/withdrawal/status{?id}` few times. Check status of withdrawals by transaction explorer 
            (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : The withdrawal must be in the `pending` state and the wallet must not send any messages.
            There is no any correlated messages in `external withdrawals` table.
- COMMENT :

6. -[x] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for TONs and check it by `/v1/withdrawal/status{?id}` 
            few times and try to catch all statuses: `pending`, `processing`, `processed`. Check status of withdrawals 
            by transaction explorer (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : Withdrawal status must correlate with explorer status. In `withdrawal_requests` DB table must be
            `processing=true`, `processed=true` and `failed=false`, `confirmed=false` in `external_withdrawals` table 
            as final state.   
- COMMENT :

7. -[x] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for Jettons with not deployed Jetton hot wallets 
            and withdrawals by `/v1/withdrawal/status{?id}` few times. Check status of withdrawals by 
            transaction explorer (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : The withdrawal must be in the `pending` state and the wallet must not send any messages.
            There is no any correlated messages in `external withdrawals` table.
- COMMENT :

8. -[x] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for Jettons with deployed Jetton hot wallets 
            and check it by `/v1/withdrawal/status{?id}` few times and try to catch all statuses: 
            `pending`, `processing`, `processed`. Check status of withdrawals by transaction explorer 
            (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : Withdrawal status must correlate with explorer status. In `withdrawal_requests` DB table must be
            `processing=true`, `processed=true` and `failed=false`, `confirmed=true` in `external_withdrawals` table
            as final state.
- COMMENT :

9. -[x] Checked
- TEST    : Start the service after some downtime. Check sync status by `/v1/system/sync` few times. 
- RESULT  : Start status should be `"is_synced": false` then become `"is_synced": true`
- COMMENT :

10. -[x] Checked
- TEST    : Start service with `IS_TESTNET=true` env var. Make some withdrawals by `/v1/withdrawal/send` method to
            TESTNET and MAINNET user-friendly form addresses. 
- RESULT  : All withdrawals must be accepted
- COMMENT :

11. -[x] Checked
- TEST    : Start service with `IS_TESTNET=false` env var. Make some withdrawals by `/v1/withdrawal/send` method to
            TESTNET user-friendly form addresses.
- RESULT  : All withdrawals must be rejected
- COMMENT :

12. -[x] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method to service internal addresses 
            (hot wallet, jetton hot wallet, owner, ton deposit, jetton deposit).
- RESULT  : All withdrawals must be rejected
- COMMENT :

13. -[x] Checked
- TEST    : Try all methods with auth using wrong token
- RESULT  : All requests must be rejected
- COMMENT :

14. -[x] Checked
- TEST    : Make TON and Jetton withdrawal to -1 workchain
- RESULT  : Withdrawals must be rejected
- COMMENT :

15. -[x] Checked
- TEST    : Make withdrawals by `/v1/withdrawal/service/ton` and `/v1/withdrawal/service/jetton` from unknown address 
            and another network (testnet addr for mainnet address and -1 workchain address)
- RESULT  : Withdrawals must be rejected
- COMMENT :

16. -[x] Checked
- TEST    : Make withdrawal by `/v1/withdrawal/service/ton` and `/v1/withdrawal/service/jetton` from known internal but 
            not Jetton deposit owner and not TON deposit address (hot wallet and Jetton wallet)
- RESULT  : Withdrawal must be rejected
- COMMENT :

17. -[x] Checked
- TEST    : Make TON withdrawal by `/v1/withdrawal/service/ton` from known Jetton deposit owner address.
            Check for unusual transactions in the database
- RESULT  : Withdrawal must be accepted. All TONs must be sent from Jetton deposit owner to hot wallet. There is no
            any deposit transactions (incomes/withdrawals) in the database. In `service_withdrawal_request` DB table
            must be `processed = true`
- COMMENT :

18. -[x] Checked
- TEST    : Make Jetton (not deposit Jetton type) withdrawal by `/v1/withdrawal/service/jetton` from known internal
            Jetton deposit owner address. Check for unusual transactions in the database
- RESULT  : Withdrawal must be accepted. Jettons must be sent from Jetton wallet to hot wallet.
            There is no any deposit transactions (incomes/withdrawals) in the database.
            In `service_withdrawal_request` DB table must be `processed = true`
- COMMENT : Should be zero forward TON amount in transfer message to prevent invoking notification message and 
            incorrect interpretation hot wallet incoming message

19. -[x] Checked
- TEST    : Make Jetton withdrawal by `/v1/withdrawal/service/jetton` from known internal TON deposit address. 
            Check for unusual transactions in the database
- RESULT  : Withdrawal must be accepted. First, there must be a TON filling transaction from hot wallet to TON deposit. 
            The status in `service_withdrawal_request` DB should be changed to `filled=true`. Later, the Jettons should
            be sent from Jetton wallet to the hot wallet and in DB table should be `processed = true` and `filled=false`.
            There is no any deposit transactions (incomes/withdrawals) in the database. There should be audit log 
            warning about withdrawal from TON deposit to non-hot wallet. 
- COMMENT : The balance on the deposit side is calculated correctly, but the withdrawal of Jettons (and TONs) from the 
            TON deposit occurs through the Jetton wallet and is not detected by the block scanner as an internal TON 
            withdrawal. The deposit balance on the hot wallet side is not replenished.

20. -[x] Checked
- TEST    : Make Jetton (for deposit Jetton type) withdrawal by `/v1/withdrawal/service/jetton` from known internal
            Jetton deposit owner address.
- RESULT  : In `service_withdrawal_request` DB table should be `processed = true` and `filled=false` with zero balances.
            Must be warning about rejected withdrawal in audit log
- COMMENT :

21. -[x] Checked
- TEST    : Make TON withdrawal by `/v1/withdrawal/service/ton` from Jetton owner address with zero TON balance
- RESULT  : Withdrawal must be accepted. There should be audit log info about zero balance.
            There is no any deposit transactions (incomes/withdrawals) in the database and no messages from hot wallet.
            In `service_withdrawal_request` DB table must be `processed = true`
- COMMENT :

22. -[x] Checked
- TEST    : Make Jetton withdrawal by `/v1/withdrawal/service/jetton` from Jetton owner address and from TON 
            deposit address with zero Jetton balance
- RESULT  : Withdrawal must be accepted. There should be audit log info about zero balance. There is no any deposit 
            transactions (incomes/withdrawals) in the database and no messages from hot wallet. 
            In `service_withdrawal_request` DB table must be `processed = true`
- COMMENT :

23. -[x] Checked
- TEST    : Set some Jetton in `JETTONS` env variable. Start service to init jetton hot wallet in DB. 
            Remove Jetton from env variable and restart. Try `/v1/address/new`, `/v1/withdrawal/send` for removed Jetton.
- RESULT  : Must be currency error for `/v1/address/new`, `/v1/withdrawal/send`.
- COMMENT :

24. -[ ] Checked
- TEST    : Set some Jetton in `JETTONS` env variable. Start service to init jetton hot wallet in DB.
            Remove Jetton from env variable and restart. Try `/v1/address/all`, `/v1/income` for user 
            with removed Jetton deposits.
- RESULT  : Removed Jetton should not appear in `/v1/address/all`, `/v1/income`.
- COMMENT : Not implemented yet

25. -[x] Checked
- TEST    : Make some payments at deposits and check it by `/v1/history{?user_id,currency,limit,offset}` method
            with different `DEPOSIT_SIDE_BALANCE` env var
- RESULT  : Incomes must correlate with payments and DB `external_incomes` table. The history on the deposits side
            should always be displayed.
- COMMENT :

26. -[x] Checked
- TEST    : Replenish the TON deposit from the masterchain wallet and check it by
            `/v1/history{?user_id,currency,limit,offset}` method.
- RESULT  : The sender's address must be displayed correctly in the history.
- COMMENT :

27. -[x] Checked
- TEST    : Replenish the TON deposit (when it in nonexist status) with a bounceable message and check it by
            `/v1/history{?user_id,currency,limit,offset}` method. Also check logs.
- RESULT  : The bounced payment should not be in the history. There should be no errors in the logs, only a warning
            about a bounced message.
- COMMENT :

28. -[x] Checked
- TEST    : Replenish the Jetton deposit with zero forward amount and check it by
            `/v1/history{?user_id,currency,limit,offset}` method.
- RESULT  : The sender's address must be not presented in the history.
- COMMENT :

### Internal logic

1. -[x] Checked
- TEST    : Replenish the deposit with TONs and Jettons so that as a result the amount on the hot wallet is greater 
            than `hot_wallet_max_balance` when cold wallet is not active and cold wallet address in non-bounceable format. 
            Check withdrawals in DB
- RESULT  : You must find new withdrawal in `withdrawal_requests` table with `is_internal=true` and `bounceable=false`. 
            And final status must correlate with explorer. 
- COMMENT :

2. -[ ] Checked
- TEST    : Start the service while a workchain merges and splits. Check the integrity of the chain of blocks in the 
            table by comparing with the explorer.
- RESULT  : There should be no missing blocks in the DB.
- COMMENT :

3. -[x] Checked
- TEST    : Replenish the deposit with TONs and Jettons so that as a result the amount on the hot wallet is greater
            than `hot_wallet_max_balance`. Try with and without `hot_wallet_residual_balance` parameter. Check withdrawals in DB
- RESULT  : You must find new withdrawal in `withdrawal_requests` table with `is_internal=true`. And final status
            must correlate with explorer. Withdrawal amount must correlate with hysteresis formula 
            (and `hot_wallet_residual_balance` parameter).
- COMMENT :

### Deploy

1. -[x] Checked
- TEST    : Build docker images and start `payment-postgres`, `payment-processor` services using README.md instructions
            Check availability and functionality of service.
- RESULT  : The API must be accessible and functional
- COMMENT :

2. -[x] Checked
- TEST    : Start optional `payment-grafana` service using README.md instructions
            Check availability and functionality of service.
- RESULT  : The `payments` Grafana dashboard must be accessible and show DB data
- COMMENT :

3. -[x] Checked
- TEST    : Start `payment-processor` with `QUEUE_ENABLED=true` env var and optional `payment-rabbitmq` service 
            using README.md instructions. Make some payments to deposits. Check availability and functionality of 
            service by RabbitMQ dashboard
- RESULT  : Must be some message activity in RabbitMQ dashboard for exchange
- COMMENT :

4. -[x] Checked
- TEST    : Start `payment-test` service using technical_notes.md instructions 
            with `CIRCULATION=false` env variable. Check availability and functionality of service by Grafana dashboard.
- RESULT  : Grafana must show prometheus metrics from `payment-test` service (deposit and total balances)
- COMMENT :

5. -[x] Checked
- TEST    : Start `payment-test` service using technical_notes.md instructions
            with `CIRCULATION=true` env variable. Check availability and functionality of service by Grafana dashboard.
- RESULT  : Grafana must show prometheus metrics from `payment-test` service (deposit and total balances) and 
            payment activity
- COMMENT :

6. -[x] Checked
- TEST    : Start `payment-processor` with `WEBHOOK_ENDPOINT=http://localhost:3333/webhook` env var.
            Start test webserver from `cmd/testwebhook/main.go`. Make some payments to deposits. Check payments data 
            at webserver side. Add env variable `WEBHOOK_TOKEN=123` and restart `payment-processor`. Make some payments 
            to deposits. Check payments data at webserver side.  
- RESULT  : Must be payments log activity in webserver and warning about the absence of a token when the variable 
            `WEBHOOK_TOKEN` is not set.
- COMMENT :

7. -[x] Checked
- TEST    : Start `payment-processor` with webhooks. Make Jetton payment to deposits with zero froward amount. 
            Check payments data at webserver side.
- RESULT  : The sender's address must be not presented.
- COMMENT :

### Stability test

1. -[x] Checked
- TEST    : Start `payment-test` service using technical_notes.md instructions
            with `CIRCULATION=true` env variable for long time (with enough amount of test TONs on wallet). 
            Periodically check availability and functionality of service by Grafana dashboard and docker logs.
- RESULT  : There should be no abnormal behavior of service and errors in log
- COMMENT : Add reconnect when timeout expires

### Highload test

1. -[ ] Checked
- TEST    : Start `payment-test` service using technical_notes.md instructions
            with `CIRCULATION=true` env variable and depositsQty = 100 x 3 types of deposits 
            (with enough amount of test TONs on wallet).
            Periodically check availability and functionality of service by Grafana dashboard and docker logs.
- RESULT  : There should be no abnormal behavior of service and errors in log
- COMMENT :