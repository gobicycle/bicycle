package core

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/tvm"
	"github.com/tonkeeper/tongo/wallet"
	"math"
	"sync"
)

// TODO: check for context.Background for entire code
// TODO: wrap errors with text for entire code

type Jetton struct {
	Master tongo.AccountID
	Code   string
	Data   string
}

// DepositGenerator is a thread-safe generator for TON and Jetton deposits.
type DepositGenerator struct {
	db    storage
	shard tongo.ShardID
	mutex sync.Mutex

	ownerAddress     tongo.AccountID   // address for wallet - owner of Jetton deposit proxy
	publicKey        ed25519.PublicKey // public key for TON deposits wallet (to control all deposits with one private key)
	jettons          map[string]Jetton
	blockchainConfig string // it is preferable to use a trimmed config to increase speed
}

// Options configures behavior of a DepositGenerator instance.
type Options struct {
	// TODO: implement
	jettons map[string]Jetton
}

type Option func(o *Options)

func WithJettons(jettons map[string]Jetton) Option {
	return func(o *Options) {
		o.jettons = jettons
	}
}

func NewDepositGenerator(
	storage storage,
	shard tongo.ShardID,
	ownerAddress tongo.AccountID,
	publicKey ed25519.PublicKey,
	blockchainConfig string,
	opts ...Option,
) (*DepositGenerator, error) {
	// TODO: implement
	options := &Options{}
	for _, o := range opts {
		o(options)
	}

	res := DepositGenerator{
		db:               storage,
		shard:            shard,
		ownerAddress:     ownerAddress,
		publicKey:        publicKey,
		blockchainConfig: blockchainConfig,
	}
	// TODO: check other options
	if options.jettons != nil {
		res.jettons = options.jettons
	}

	return &res, nil
}

func (g *DepositGenerator) GenerateTonDeposit(ctx context.Context, userID string) (*tongo.AccountID, error) {
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

	proxy, jettonWalletAddr, err := g.generateDepositJettonWalletForProxy(ctx, currency, lastSubWalletID+1)
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
func generateTonDepositWallet(
	pubKey ed25519.PublicKey,
	shard tongo.ShardID,
	startSubWalletID uint32,
) (
	*tongo.AccountID,
	uint32,
	error,
) {
	for id := startSubWalletID; id < math.MaxUint32; id++ {
		subWalletID := int(id) // TODO: check for max value
		// TODO: check for testnet or mainnet subwallet IDs
		addr, err := wallet.GenerateWalletAddress(pubKey, wallet.V3R2, DefaultWorkchainID, &subWalletID)
		if err != nil {
			return nil, 0, err
		}
		if shard.MatchAccountID(addr) {
			return &addr, id, nil
		}
	}
	return nil, 0, ErrSubwalletNotFound
}

// generateDepositJettonWalletForProxy
// Generates jetton wallet address for custom shard and proxy contract as owner with subwallet_id >= startSubWalletId
func (g *DepositGenerator) generateDepositJettonWalletForProxy(
	ctx context.Context,
	currency string,
	startSubWalletID uint32,
) (
	proxy *JettonProxy,
	addr *tongo.AccountID,
	err error,
) {

	jetton, ok := g.jettons[currency]
	if !ok {
		return nil, nil, ErrJettonNotFound
	}

	emulator, err := tvm.NewEmulatorFromBOCsBase64(jetton.Code, jetton.Data, g.blockchainConfig, tvm.WithBalance(DefaultBalanceForEmulation))
	if err != nil {
		return nil, nil, err
	}

	for id := startSubWalletID; id < math.MaxUint32; id++ {
		proxy, err = NewJettonProxy(id, g.ownerAddress)
		if err != nil {
			return nil, nil, err
		}
		jettonWalletAddress, err := getJettonWalletAddressByTVM(ctx, proxy.Address(), jetton.Master, emulator)
		if err != nil {
			return nil, nil, err
		}
		if g.shard.MatchAccountID(*jettonWalletAddress) {
			// TODO: testnet flag
			//addr = jettonWalletAddress.ToTonutilsAddressStd(0)
			//addr.SetTestnetOnly(config.Config.Testnet)
			return proxy, jettonWalletAddress, nil
		}
	}
	return nil, nil, fmt.Errorf("jetton wallet address not found")
}

func getJettonWalletAddressByTVM(
	ctx context.Context,
	owner, jettonMaster tongo.AccountID,
	emulator *tvm.Emulator,
) (
	*tongo.AccountID,
	error,
) {

	slice, err := tlb.TlbStructToVmCellSlice(owner.ToMsgAddress())
	if err != nil {
		return nil, err
	}

	errCode, stack, err := emulator.RunSmcMethod(ctx, jettonMaster, "get_wallet_address", tlb.VmStack{slice})
	if err != nil {
		return nil, err
	}

	if errCode != 0 && errCode != 1 {
		return nil, fmt.Errorf("method execution failed with code: %v", errCode)
	}
	if len(stack) != 1 || stack[0].SumType != "VmStkSlice" {
		return nil, fmt.Errorf("ivalid stack value")
	}

	var msgAddress tlb.MsgAddress
	err = stack[0].VmStkSlice.UnmarshalToTlbStruct(&msgAddress)
	if err != nil {
		return nil, err
	}

	addr, err := tongo.AccountIDFromTlb(msgAddress)
	if err != nil {
		return nil, err
	}

	if addr.Workchain != DefaultWorkchainID {
		return nil, fmt.Errorf("not default workchain for jetton wallet address")
	}
	if addr == nil {
		return nil, fmt.Errorf("addres none")
	}
	return addr, nil
}
