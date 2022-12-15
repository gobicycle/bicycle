## Manual testing plan for v0.1.0
Template:
-[x] Checked
- TEST    : test description
- RESULT  : expected result
- COMMENT : some comment 

### Initialization

1. -[ ] Checked
- TEST    : Run with not deployed (and zero balance) hot wallet (new seed phrase)
- RESULT  : There must be an insufficient balance error
- COMMENT : 

2. -[ ] Checked
- TEST    : Run with uninit hot wallet with balance > minimum balance
- RESULT  : Hot wallet must be initialized at first start of service
- COMMENT :

3. -[ ] Checked
- TEST    : Run with new seed phrase when hot wallet already exist in DB
- RESULT  : There must be an incorrect seed phrase error
- COMMENT :

4. -[ ] Checked
- TEST    : Run service with empty DB and stop after few minutes. Check time of first and last block 
            in `block_data` table
- RESULT  : Time must correlate with system time
- COMMENT :

5. -[ ] Checked
- TEST    : Run with nonexist hot jetton wallet and receive external jetton transfer at jetton deposit 
            (> minimal withdrawal amount)
- RESULT  : Jetton hot wallet must be initialized by Jetton withdrawal from deposit, 
            if jetton deposit successfully initialized (it depends on transfer sender)
- COMMENT :

### API

1. -[ ] Checked
- TEST    : Use `/v1/address/new` method (few for TONs and few for Jettons for different users). 
            Check new addresses in DB
- RESULT  : You must receive different addresses in user-friendly format with `bounce = false` flag and 
            testnet flag correlated with `IS_TESTNET` env var and raw in DB
- COMMENT :

2. -[ ] Checked
- TEST    : Use `/v1/address/all{?user_id}` method and compare with addresses created at 1. And check it by DB
- RESULT  : All addresses must be received
- COMMENT :

3. -[ ] Checked
- TEST    : Check `/v1/balance{?user_id}` for new empty deposits
- RESULT  : Balances must be zero
- COMMENT :

4. -[ ] Checked
- TEST    : Make some payments at deposits and check it by `/v1/balance{?user_id}` method 
            with different `DEPOSIT_SIDE_BALANCE` env var
- RESULT  : Balances must correlate with payments
- COMMENT :

5. -[ ] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for TONs and Jettons with amount > hot wallet balance
            and check it by `/v1/withdrawal/status{?id}` few times. Check status of withdrawals by transaction explorer 
            (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : The withdrawal must be in the `pending` state and the wallet must not send any messages.
            There is no any correlated messages in `external withdrawals` table.
- COMMENT :

6. -[ ] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for TONs and check it by `/v1/withdrawal/status{?id}` 
            few times and try to catch all statuses: `pending`, `processing`, `processed`. Check status of withdrawals 
            by transaction explorer (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : Withdrawal status must correlate with explorer status. In `withdrawal_requests` DB table must be
            `processing=true`, `processed=true` and `failed=false`, `confirmed=false` in `external_withdrawals` table 
            as final state.   
- COMMENT :

7. -[ ] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for Jettons with not deployed Jetton hot wallets 
            and withdrawals by `/v1/withdrawal/status{?id}` few times. Check status of withdrawals by 
            transaction explorer (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : The withdrawal must be in the `pending` state and the wallet must not send any messages.
            There is no any correlated messages in `external withdrawals` table.
- COMMENT :

8. -[ ] Checked
- TEST    : Make some withdrawals by `/v1/withdrawal/send` method for Jettons with deployed Jetton hot wallets 
            and check it by `/v1/withdrawal/status{?id}` few times and try to catch all statuses: 
            `pending`, `processing`, `processed`. Check status of withdrawals by transaction explorer 
            (e.g. https://testnet.tonapi.io/ or https://tonapi.io/). Check withdrawal in DB.
- RESULT  : Withdrawal status must correlate with explorer status. In `withdrawal_requests` DB table must be
            `processing=true`, `processed=true` and `failed=false`, `confirmed=true` in `external_withdrawals` table
            as final state.
- COMMENT :

9. -[ ] Checked
- TEST    : Start the service after some downtime. Check sync status by `/v1/system/sync` few times. 
- RESULT  : Start status should be `"is_synced": false` then become `"is_synced": true`
- COMMENT :

10. -[ ] Checked
- TEST    : Start service with `IS_TESTNET=true` env var. Make some withdrawals by `/v1/withdrawal/send` method to
            TESTNET and MAINNET user-friendly form addresses. 
- RESULT  : All withdrawals must be accepted
- COMMENT :

11. -[ ] Checked
- TEST    : Start service with `IS_TESTNET=false` env var. Make some withdrawals by `/v1/withdrawal/send` method to
            TESTNET user-friendly form addresses.
- RESULT  : All withdrawals must be rejected
- COMMENT :

12. -[ ] Checked
- TEST    : Try all methods with auth using wrong token
- RESULT  : All requests must be rejected
- COMMENT :

13. -[ ] Checked
- TEST    : Make TON and Jetton withdrawal to -1 and 1 workchain
- RESULT  : Withdrawals must be rejected
- COMMENT :

[//]: # (TODO: fix for new API method)
[//]: # (14. -[ ] Checked)

[//]: # (- TEST    : Make withdrawals by `/v1/withdrawal/service` from unknown address and another network )

[//]: # (            &#40;testnet addr for mainnet&#41; address and -1 workchain address)

[//]: # (- RESULT  : Withdrawals must be rejected)

[//]: # (- COMMENT :)

[//]: # ()
[//]: # (15. -[ ] Checked)

[//]: # (- TEST    : Make withdrawal by `/v1/withdrawal/service` from known internal not Jetton deposit owner address &#40;hot wallet&#41;)

[//]: # (- RESULT  : Withdrawal must be rejected)

[//]: # (- COMMENT :)

[//]: # ()
[//]: # (16. -[ ] Checked)

[//]: # (- TEST    : Make full TON withdrawal by `/v1/withdrawal/service` from known internal Jetton deposit owner address.)

[//]: # (            Check for unusual transactions in the database)

[//]: # (- RESULT  : Withdrawal must be accepted. All TONs must be sent from Jetton deposit owner to hot wallet. There is no )

[//]: # (            any deposit transactions &#40;incomes/withdrawals&#41; in the database. In `service_withdrawal_request` DB table)

[//]: # (            must be `processed = true`)

[//]: # (- COMMENT :)

[//]: # ()
[//]: # (17. -[ ] Checked)

[//]: # (- TEST    : Make TON and Jetton &#40;not deposit Jetton type&#41; withdrawal by `/v1/withdrawal/service` from known internal )

[//]: # (            Jetton deposit owner address. Jetton amount must be equal Jetton balance. Ton amount must be near )

[//]: # (            TON balance &#40;with fee correction&#41;. Check for unusual transactions in the database)

[//]: # (- RESULT  : Withdrawal must be accepted. TON and Jettons must be sent from Jetton deposit owner to hot wallet. )

[//]: # (            There is no any deposit transactions &#40;incomes/withdrawals&#41; in the database. )

[//]: # (            In `service_withdrawal_request` DB table must be `processed = true`)

[//]: # (- COMMENT :)

[//]: # ()
[//]: # (18. -[ ] Checked)

[//]: # (- TEST    : Make Jetton &#40;for deposit Jetton type&#41; withdrawal by `/v1/withdrawal/service` from known internal)

[//]: # (            Jetton deposit owner address.)

[//]: # (- RESULT  : Withdrawal must be not processed. Must be warning in audit log)

[//]: # (- COMMENT :)

[//]: # ()
[//]: # (19. -[ ] Checked)

[//]: # (- TEST    : Make TON withdrawal &#40;with JettonMaster address provided&#41; by `/v1/withdrawal/service` from known internal)

[//]: # (            Jetton deposit owner address)

[//]: # (- RESULT  : Withdrawal must be accepted. TONs must be sent from Jetton deposit owner to hot wallet.)

[//]: # (            There is no any deposit transactions &#40;incomes/withdrawals&#41; in the database.)

[//]: # (            In `service_withdrawal_request` DB table must be `processed = true`)

[//]: # (- COMMENT :)

### Internal logic

1. -[ ] Checked
- TEST    : Replenish the deposit with TONs and Jettons so that as a result the amount on the hot wallet is greater 
            than hot_wallet_max_balance. Check withdrawals in DB
- RESULT  : You must find new withdrawal in `withdrawal_requests` table with `is_internal=true`. And final status 
            must correlate with explorer. 
- COMMENT :

### Deploy

1. -[ ] Checked
- TEST    : Build docker images and start `payment-postgres`, `payment-processor` services using README.md instructions
            Check availability and functionality of service.
- RESULT  : The API must be accessible and functional
- COMMENT :

2. -[ ] Checked
- TEST    : Start optional `payment-grafana` service using README.md instructions
            Check availability and functionality of service.
- RESULT  : The `payments` Grafana dashboard must be accessible and show DB data
- COMMENT :

3. -[ ] Checked
- TEST    : Start `payment-processor` with `QUEUE_ENABLED=true` env var and optional `payment-rabbitmq` service 
            using README.md instructions. Make some payments to deposits. Check availability and functionality of 
            service by RabbitMQ dashboard
- RESULT  : Must be some message activity in RabbitMQ dashboard for exchange
- COMMENT :

4. -[ ] Checked
- TEST    : Start optional `payment-prometheus`, `payment-test` services using README.md instructions 
            with `CIRCULATION=false` env variable. Check availability and functionality of service by Grafana dashboard.
- RESULT  : Grafana must show prometheus metrics from `payment-test` service (deposit and total balances)
- COMMENT :

5. -[ ] Checked
- TEST    : Start optional `payment-prometheus`, `payment-test` services using README.md instructions
            with `CIRCULATION=true` env variable. Check availability and functionality of service by Grafana dashboard.
- RESULT  : Grafana must show prometheus metrics from `payment-test` service (deposit and total balances) and 
            payment activity
- COMMENT :

### Stability test

1. -[ ] Checked
- TEST    : Start optional `payment-prometheus`, `payment-test` services using README.md instructions
            with `CIRCULATION=true` env variable for long time (with enough amount of test TONs on wallet). 
            Periodically check availability and functionality of service by Grafana dashboard and docker logs.
- RESULT  : There should be no abnormal behavior of service and errors in log
- COMMENT :

### Highload test

1. -[ ] Checked
- TEST    : Make a lot of payments and withdrawals (more than highload wallet message capacity) at the same time
- RESULT  : There should be no abnormal behavior of service and errors in log
- COMMENT :