# Jettons compatibility information

**! Be careful with Jettons metadata, the information may not be up-to-date, see the current one in the explorer or https://github.com/tonkeeper/ton-assets**

## TGR

### mainnet
- Address: 0:2F0DF5851B4A185F5F63C0D0CD0412F5ACA353F577DA18FF47C936F99DBD849A
- Code hash: 394e236fdaa478d218aa923b06dc0d3ad08ce65cbce5b0ab3ead62c61da69920
- Metadata:
  * name: Tegro
  * symbol: TGR
  * description: Cross-platform DeFi ecosystem Tegro
  * image: https://tegro.io/tgr.png
- Tested with payment processor:
    [x] Deposit filling from external address and withdrawal to hot wallet
    [x] Withdrawal to new external address with wallet init
    [] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: yes
- Comments: sends notification with bounce flag

### testnet
- Address: 0:8AB7658F197F9F874708033DFC2E377F38A423A1913F72A93B42B606B0D3FAC1
- Code hash: failed to calc via tonutils
- Metadata:
  * name: Tegro
  * symbol: TGR
  * description: Cross-platform payment token on the TON blockchain.
  * image: https://tegro.io/tgr.png
- Tested with payment processor:
  [x] Deposit filling from external address and withdrawal to hot wallet
  [x] Withdrawal to new external address with wallet init
  [x] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: no
- Comments: code not equal to actual TGR jetton in mainnet! Jetton with the equivalent code was not found in the testnet.

## SCALE

### mainnet
- Address: 0:65aac9b5e380eae928db3c8e238d9bc0d61a9320fdc2bc7a2f6c87d6fedf9208
- Code hash: c81828dcd4df2a3a7516762c154dd973b92a26408db3bc143c7adda0576a9d6c
- Metadata:
  * name: Scaleton
  * symbol: SCALE
  * description: SCALE is a utility token that will be used to support all independent developers.
  * image: ipfs://QmSMiXsZYMefwrTQ3P6HnDQaCpecS4EWLpgKK5EX1G8iA8
- Tested with payment processor:
  [] Deposit filling from external address and withdrawal to hot wallet
  [] Withdrawal to new external address with wallet init
  [] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: yes
- Comments:

### testnet
- Address: 0:1b310ceeb868829ded11ed0123f67ad7b2333b80d8de566ae5059a2ec82f9208
- Code hash: f95ba0330b38cdf3459b1e811e5fc6fa6cfee566d7b764455c0468140365a737
- Metadata:
  * name: Scaleton
  * symbol: SCALE
  * description:
  * image: ipfs://QmSMiXsZYMefwrTQ3P6HnDQaCpecS4EWLpgKK5EX1G8iA8
- Tested with payment processor:
  [x] Deposit filling from external address and withdrawal to hot wallet
  [x] Withdrawal to new external address with wallet init
  [x] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: no
- Comments: code not equal to actual SCALE jetton in mainnet!

## FNZ

### mainnet
- Address: 0:C224BD22407A1F70106F1411DD546DB7DD18B657890234581F453FA0A8077739
- Code hash: b4804ee49db9823eb5e9bdd98ff3784913c5dd01762dfd6e257f15be97b9fcf4
- Metadata:
    * name: Fanzee Token 
    * symbol: FNZ
    * description: fanz.ee is a web3 fan engagement platform designed to help sports and entertainment organisations meaningfully connect with their fans through immersive gamification experiences
    * image: https://media.fanz.ee/images/91ee938a92934656a01131c569b377b6.png
    * decimals: 9
- Tested with payment processor:
  [x] Deposit filling from external address and withdrawal to hot wallet
  [x] Withdrawal to new external address with wallet init
  [] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: yes
- Comments: sends notification with non-bounce flag

### testnet
- Address: 0:FCEE09C7BA28DDBA7B3F83EEBC6EC1B36B3AA3E40B98C3A69372DC1EA711353D
- Code hash: b4804ee49db9823eb5e9bdd98ff3784913c5dd01762dfd6e257f15be97b9fcf4
- Metadata:
    * name: FANZEE test Coin
    * symbol: FNZT
    * description: This is an example jetton for the TON network
    * image: https://media.fanz.ee/images/91ee938a92934656a01131c569b377b6.png
    * decimals: 9
- Tested with payment processor:
  [] Deposit filling from external address and withdrawal to hot wallet
  [] Withdrawal to new external address with wallet init
  [] Highload test
- Presented in https://github.com/tonkeeper/ton-assets: no
- Comments: code is equal with the mainnet jetton.
