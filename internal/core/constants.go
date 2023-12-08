package core

import "github.com/tonkeeper/tongo/wallet"

const (
	TonSymbol                  = "TON"
	DefaultWorkchainID         = 0 // use only 0 workchain
	MasterchainID              = -1
	DefaultShard               = -9223372036854775808 // 0x8000000000000000 include all shards
	DefaultBalanceForEmulation = 1_000_000_000        // nanotons
	DefaultTonDepositWallet    = wallet.V3R2
)
