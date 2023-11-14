package core

import (
	"context"
	"crypto/ed25519"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/wallet"
	"math"
	"sync"
)

// TODO: use preloaded data and code for Jettons and generate all offline. Do not use blockchain package.
// TODO: wrap errors with text

type Jetton struct {
	Master tongo.AccountID
}

// DepositGenerator is a thread-safe generator for TON and Jetton deposits.
type DepositGenerator struct {
	db         storage
	blockchain blockchain
	shard      tongo.ShardID
	mutex      sync.Mutex

	ownerAddress tongo.AccountID   // address for wallet - owner of Jetton deposit proxy
	publicKey    ed25519.PublicKey // public key for TON deposits wallet (to control all deposits with one private key)
	jettons      map[string]Jetton
}

func (g *DepositGenerator) GenerateTonDeposit(
	ctx context.Context,
	userID string,
) (
	*tongo.AccountID,
	error,
) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	lastSubWalletID, err := g.db.GetLastSubwalletID(ctx)
	if err != nil {
		return nil, err
	}

	depositAddress, subWalletID, err := generateTonDepositWallet(g.publicKey, g.shard, lastSubWalletID+1)
	if err != nil {
		return nil, err
	}

	err = g.db.SaveTonWallet(ctx,
		WalletData{
			SubwalletID: subWalletID,
			UserID:      userID,
			Currency:    TonSymbol,
			Type:        TonDepositWallet,
			Address:     depositAddress.Address, // TODO: use tongo Address
		},
	)
	if err != nil {
		return nil, err
	}

	return depositAddress, nil
}

func (g *DepositGenerator) GenerateJettonDeposit(
	ctx context.Context,
	userID string,
	currency string,
) (
	*tongo.AccountID,
	error,
) {
	g.mutex.Lock()
	defer g.mutex.Unlock()

	lastSubWalletID, err := g.db.GetLastSubwalletID(ctx)
	if err != nil {
		return nil, err
	}

	jetton, ok := g.jettons[currency]
	if !ok {
		return nil, ErrJettonNotFound
	}

	proxy, jettonWalletAddr, err := g.blockchain.GenerateDepositJettonWalletForProxy(ctx, g.shard, g.ownerAddress, jetton.Master, lastSubWalletID+1)
	if err != nil {
		return nil, err
	}

	err = g.db.SaveJettonWallet(
		ctx,
		proxy.address.Address, // TODO: use tongo address
		WalletData{
			UserID:      userID,
			SubwalletID: proxy.SubwalletID,
			Currency:    currency,
			Type:        JettonDepositWallet,
			Address:     jettonWalletAddr.Address, // TODO: use tongo address
		},
		false,
	)
	if err != nil {
		return nil, err
	}

	return &proxy.address, nil
}

// generateTonDepositWallet generates V3R2 subwallet for DefaultWorkchain, custom shard and
// subWalletID >= startSubWalletID and returns wallet address and new subWalletID
func generateTonDepositWallet(pubKey ed25519.PublicKey, shard tongo.ShardID, startSubWalletID uint32) (
	*tongo.AccountID,
	uint32,
	error,
) {
	for id := startSubWalletID; id < math.MaxUint32; id++ {
		subWalletID := int(id) // TODO: check for max value
		// TODO: check for testnet or mainnet subwallet IDs
		addr, err := wallet.GenerateWalletAddress(pubKey, wallet.V3R2, DefaultWorkchain, &subWalletID)
		if err != nil {
			return nil, 0, err
		}
		if shard.MatchAccountID(addr) {
			return &addr, id, nil
		}
	}
	return nil, 0, ErrSubwalletNotFound
}
