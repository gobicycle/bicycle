### List of possible vulnerabilities
In the format:
- P: problem
- T: threat
- S: possible solutions
- D: decision

#### Untrusted node
- P: unreliable data from the node lite server
- T: the node operator can control the behavior of the service
- S: use your trusted node or use a web3 library with proof checking (like tolibgo)
- D: add a recommendation for launching your node in the readme

#### Time out of sync
- P: time out of sync between a node and service
- T: incorrect expiration check and double withdrawals
- S: use your trusted node and check time diff between node and service
- D: add a recommendation for launching your node in the readme and service stops if time diff too big

#### Blockchain out of sync
- P: out of sync between a blockchain and service (by blocks)
- T: the service may mark some transactions that have not yet been found as expired and make double withdrawals
- S: check time diff between last processed block and actual time (time of last block)
- D: service checks time diff between last the processed block and the actual time and does not make any withdrawals until service is synchronized

#### Repeated withdrawal of TONs from the deposit
- P: when a message is sent to wallet with 128+32 mode, the wallet contract is deleted, but the message itself remains in the node's mempool for some time. 
If TONs arrive at the wallet address at this time, the message will be applied again (because seqno is being reset).
- T: repeated withdrawal of TONs from the deposit to the hot wallet ignoring the cutoff for the minimum withdrawal and growth of internal fees
- S: reducing the valid_until time for message
- D: small valid_until time for message

#### Bruteforce API Token through Time Attack
- P: the api token can be bruteforced by the difference in the token verification time
- T: an attacker can control the service through the API
- S: isolation of the payment processor from the external network and use constant time function to check token 
- D: isolation recommendation in readme file and service uses constant time function to check token

#### Forgery of messages with internal service data
- P: an attacker can copy service messages with their data (like memo)
- T: incorrect operation of the service or external behavior control from the blockchain
- S: checking the addresses of the sender and recipient of the message and checking hash for some messages
- D: checking the addresses of the sender and recipient of the message and checking hash for some messages

#### Unexpected tonutils package functionality
- P: the behavior of some functions may differ from what is expected
- T: incorrect operation of the service or malicious behavior of the library
- S: open source code review or trust the tonutils library
- D: trust the tonutils library

#### Modified wallet code in the tonutils package
- P: the library may contain a modified wallet code with additional functionality
- T: unexpected behavior of the wallet in the blockchain and control of the wallet from the outside of service
- S: compare the wallet code with a trusted source and fix the library version or trust the tonutils library
- D: trust the tonutils library

#### Untrusted binary libs in tongo package
- P: the behavior of some functions (TVM emulation) may differ from what is expected
- T: incorrect operation of the service or malicious behavior of the tongo library
- S: build binary libraries from the official TON repository or trust the tongo library
- D: build binary libraries from the official TON repository

#### Jetton wallets with unexpected behavior
- P: custom tokens may have unusual behavior on blockchain
- T: incorrect calculation of Jettons balances or large internal commissions of the service
- S: use Jettons with known behavior and conforming to the standard
- D: add a recommendations for valid Jettons to readme file

#### A lot of expensive withdrawals
- P: the service is trying to make a lot of expensive withdrawals (more than wallet balance) as a result, they displace other withdrawals from the message batch of 255 messages
- T: the service stops processing external withdrawals
- S: increasing the number of withdrawals requests from the database
- D: the number of requested withdrawals requests is configured during operation

#### Service withdrawals changes incoming Jetton balance
- P: service withdrawal may be detected as negative incoming (and interprets as unknown tx)
- T: incorrect calculation of Jettons balances
- S: check dest address and not set "unknown tx" flag 
- D: check dest address and not set "unknown tx" flag 

#### DDOS and blocking requests
- P: service can not process a lot of requests or may wait for blocked request (where mutex used)
- T: the service stops processing new requests
- S: all requests must be limited by the user
- D: recommendations for requests limitations in the readme file

#### SQL injections via `comment`, `user_id` and other text fields
- P: the danger of injection through the request fields
- T: executing an arbitrary query on the database
- S: sanitize user input
- D: use `go pgx` with sanitize

#### Withdrawals to internal address
- P: it is possible to make a withdrawal to an address and then generate the same address as the deposit address, 
     thereby making a withdrawal to the internal address
- T: this can break the uniqueness of the addresses in the wallet message batch and break the checking 
     of the correspondence of incoming and outgoing messages
- S: it is rare case, warning in technical notes, uniqueness check in withdrawal processor
- D: warning in technical notes, uniqueness check in withdrawal processor

#### Duplicate randomly generated UUIDs
- P: DB error for not unique UUID in internal or service withdrawals
- T: service crashes with fatal error
- S: the probability to find a duplicate within 103 trillion version-4 UUIDs is one in a billion
- D: the probability of error is too small, no action is required

#### Freezing and deleting unused account
- P: if account do not used by long time and its balance under 0 by storage fee, this account 
     freezes and then deletes by node
- T: if Jetton wallet do not used by a long time it may be dropped by node, data with Jetton balance is cleaning
- S: all Jettons (> cutoff) withdraws from deposits when service works normally. 
     It is dangerous for Jetton cold/hot wallets, that do not use for a long time.
     Needs to check and fill balances periodically. 
- D: recommendation in technical_notes file to periodically check and fill TON balances on hot and cold wallets.
     Recommendation to use software to do it automatically.

#### Deposit side vs hot side balance shifting when service withdrawals
- P: service withdrawal of Jettons from the TON deposit occurs through the Jetton wallet and is not detected by the 
     block scanner as an internal TON withdrawal
- T: TON balance shift between deposit side and hot wallet side
- S: use service withdrawal of Jettons from TON deposit wallet only with zero or near-zero TON balance
- D: warning about this behavior in technical_notes file for method description

#### Setting the value to "expired" without taking into account the allowable delay
- P: it is impossible to absolutely precisely synchronize in time with the blockchain, so there is an 
     allowable time delay value. If you get into this gap, the "expired" may be incorrectly set.
- T: double spending for external withdrawals or unnecessary internal withdrawals
- S: check expiration taking into account time delay
- D: check expiration taking into account time delay

#### Repetitive failed transactions burning fees
- P: with periodic withdrawal cycles, there may be situations where the transaction fails every time. 
     For example, when withdrawing to an uninitialized cold wallet with the bounce flag.
- T: constant burning of a certain amount of TON on fees
- S: additional checks to predict the success of the transaction and additional messages in the audit log
- D: made an additional check on the state of the cold wallet and checking the bounce flag for withdrawal

#### Too frequent withdrawals from a hot wallet to a cold wallet
- P: if you set only the maximum cutoff for funds on the hot wallet, then the withdrawal to the cold wallet will occur 
     if this amount is exceeded, even if the amount of the excess is less than the amount of the withdrawal fee
- T: there may be withdrawals of the amount of funds at which the amount of funds is unreasonably small, 
     which will lead to unnecessary burning of funds on fees
- S: it is necessary to set some delta between the amount of triggering the withdrawal to the cold wallet and the 
     amount that will remain after the withdrawal
- D: one more parameter has been added to the cutoffs - `hot_wallet_residual_balance`