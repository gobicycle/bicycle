package core

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"github.com/gobicycle/bicycle/internal/audit"
	"github.com/gobicycle/bicycle/internal/config"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/abi"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
	"math/big"
	"time"
)

type Wallets struct {
	Shard            tongo.ShardID
	TonHotWallet     *wallet.Wallet
	TonBasicWallet   *wallet.Wallet // basic V3 wallet to make other wallets with different subwallet_id
	JettonHotWallets map[string]JettonWallet
	PublicKey        ed25519.PublicKey
}

// InitWallets
// Generates highload hot-wallet and map[currency]JettonWallet Jetton wallets, and saves to DB
// TON highload hot-wallet (for seed and default subwallet_id) must be already active for success initialization.
func InitWallets(
	ctx context.Context,
	db storage,
	bc blockchain,
	seed string,
	jettons map[string]config.Jetton,
	shardPrefixLen int,
	hotWalletMinBalance uint64,
) (Wallets, error) {
	tonHotWallet, shard, subwalletId, err := initTonHotWallet(ctx, db, bc, seed, shardPrefixLen, hotWalletMinBalance)
	if err != nil {
		return Wallets{}, err
	}

	tonBasicWallet, _, err := bc.GenerateDefaultWallet(seed, false)
	if err != nil {
		return Wallets{}, err
	}
	// don't set TTL here because spec is not inherited by GetSubwallet method

	jettonHotWallets := make(map[string]JettonWallet)
	for currency, j := range jettons {
		w, err := initJettonHotWallet(ctx, db, bc, tonHotWallet.GetAddress(), j.Master, currency, subwalletId)
		if err != nil {
			return Wallets{}, err
		}
		jettonHotWallets[currency] = w
	}

	return Wallets{
		Shard:            *shard,
		TonHotWallet:     tonHotWallet,
		TonBasicWallet:   tonBasicWallet,
		JettonHotWallets: jettonHotWallets,
		// TODO: return pubkey
	}, nil
}

func initTonHotWallet(
	ctx context.Context,
	db storage,
	bc blockchain,
	seed string,
	shardPrefixLen int,
	hotWalletMinBalance uint64,
) (
	*wallet.Wallet,
	*tongo.ShardID,
	uint32,
	error,
) {
	tonHotWallet, subwalletId, err := bc.GenerateDefaultWallet(seed, true)
	if err != nil {
		return nil, nil, 0, err
	}

	//hotSpec := tonHotWallet.GetSpec().(*wallet.SpecHighloadV2R2)

	// TODO: set lifetime in messages!
	//hotSpec.SetMessagesTTL(uint32(config.ExternalMessageLifetime.Seconds()))

	hotWalletAccountID := tonHotWallet.GetAddress()
	//addr := AddressMustFromTonutilsAddress(tonHotWallet.Address())
	shard := ShardIDFromAccountID(hotWalletAccountID, shardPrefixLen)

	// TODO: check for shard len changes

	alreadySaved := false
	addrFromDb, err := db.GetTonHotWalletAddress(ctx)
	if err == nil && hotWalletAccountID.Address != addrFromDb {
		audit.Log(audit.Error, string(TonHotWallet), InitEvent,
			fmt.Sprintf("Hot TON wallet address is not equal to the one stored in the database. Maybe seed was being changed. %s != %s",
				hotWalletAccountID.ToHuman(true, false), addrFromDb.ToUserFormat(false))) // TODO: testnet flag
		return nil, nil, 0,
			fmt.Errorf("saved hot wallet not equal generated hot wallet. Maybe seed was being changed")
	} else if !errors.Is(err, ErrNotFound) && err != nil {
		return nil, nil, 0, err
	} else if err == nil {
		alreadySaved = true
	}

	log.Infof("Shard: %x", shard.Encode()) // TODO: format shard
	log.Infof("TON hot wallet address: %v", hotWalletAccountID.ToHuman(true, false))

	balance, status, err := bc.GetAccountCurrentState(ctx, hotWalletAccountID)
	if err != nil {
		return nil, nil, 0, err
	}

	if balance < hotWalletMinBalance {
		return nil, nil, 0,
			fmt.Errorf("hot wallet balance must be at least %d nanoTON", hotWalletMinBalance)
	}

	if status != tlb.AccountActive {
		err = bc.DeployTonWallet(ctx, tonHotWallet)
		if err != nil {
			return nil, nil, 0, err
		}
	}

	if !alreadySaved {
		err = db.SaveTonWallet(ctx, WalletData{
			SubwalletID: subwalletId,
			Currency:    TonSymbol,
			Type:        TonHotWallet,
			Address:     hotWalletAccountID.Address,
		})
		if err != nil {
			return nil, nil, 0, err
		}
	}

	return tonHotWallet, &shard, subwalletId, nil
}

func initJettonHotWallet(
	ctx context.Context,
	db storage,
	bc blockchain,
	tonHotWallet, jettonMaster tongo.AccountID,
	currency string,
	subwalletId uint32,
) (JettonWallet, error) {
	// TODO: use proxy for hot wallets
	// not init or check balances of Jetton wallets, it is not required for the service to work
	jettonWalletAccountID, err := bc.GetJettonWalletAddress(ctx, tonHotWallet, jettonMaster)
	if err != nil {
		return JettonWallet{}, err
	}
	res := JettonWallet{Address: jettonWalletAccountID, Currency: currency}
	log.Infof("%v jetton hot wallet address: %v", currency, jettonWalletAccountID.ToHuman(true, false))

	walletData, isPresented, err := db.GetJettonWallet(ctx, jettonWalletAccountID.Address)
	if err != nil {
		return JettonWallet{}, err
	}

	if isPresented && walletData.Currency == currency {
		return res, nil
	} else if isPresented && walletData.Currency != currency {
		audit.Log(audit.Error, string(JettonHotWallet), InitEvent,
			fmt.Sprintf("Hot Jetton wallets %s and %s have the same address %s",
				walletData.Currency, currency, jettonWalletAccountID.ToHuman(true, false)))
		return JettonWallet{}, fmt.Errorf("jetton hot wallet address duplication")
	}

	err = db.SaveJettonWallet(
		ctx,
		tonHotWallet.Address,
		WalletData{
			SubwalletID: subwalletId,
			Currency:    currency,
			Type:        JettonHotWallet,
			Address:     jettonWalletAccountID.Address,
		},
		true,
	)
	if err != nil {
		return JettonWallet{}, err
	}
	return res, nil
}

func buildComment(comment string) *boc.Cell {
	body := boc.NewCell()
	if err := tlb.Marshal(body, wallet.TextComment(comment)); err != nil {
		log.Fatalf("comment serialization error")
	}
	return body
}

func LoadComment(cell *boc.Cell) string {

	if cell == nil {
		return ""
	}

	if cell.BitSize() < 32 {
		return ""
	}

	op, err := cell.PickUint(32)
	if err != nil {
		log.Fatalf("op reading error")
	}
	if op != 0 {
		return ""
	}

	var comment wallet.TextComment
	err = tlb.Unmarshal(cell, &comment)
	if err != nil {
		log.Errorf("load comment error: %v", err)
		return ""
	}

	return string(comment)
}

// WithdrawTONs
// Send all TON from one wallet (and deploy it if needed) to another and destroy "from" wallet contract.
// Wallet must be not empty.
func WithdrawTONs(ctx context.Context, from, to *wallet.Wallet, comment string) error {
	if from == nil || to == nil {
		return fmt.Errorf("nil wallet")
	}
	var body *boc.Cell
	if comment != "" {
		body = buildComment(comment)
	}
	return from.Send(ctx, wallet.Message{
		Amount:  0,
		Address: to.GetAddress(),
		Body:    body,
		//Code:    *boc.Cell,
		//Data:    *boc.Cell,
		Bounce: false,
		Mode:   128 + 32, // 128 + 32 send all and destroy
	})
}

func WithdrawJettons(
	ctx context.Context,
	from, to *wallet.Wallet,
	jettonWallet tongo.AccountID,
	forwardAmount tlb.Coins,
	amount Coins,
	comment string,
) error {
	if from == nil || to == nil {
		return fmt.Errorf("nil wallet")
	}

	body := MakeJettonTransferMessage(
		to.GetAddress(),
		to.GetAddress(),
		amount.BigInt(),
		forwardAmount,
		uint64(time.Now().UnixNano()), // TODO: check for overflow in DB
		comment,
	)

	return from.Send(ctx, wallet.Message{
		Amount:  0,
		Address: jettonWallet,
		Body:    body,
		Bounce:  true,
		Mode:    128 + 32, // 128 + 32 send all and destroy
	})
}

func MakeJettonTransferMessage(
	destination, responseDest tongo.AccountID,
	amount *big.Int,
	forwardAmount tlb.Coins,
	queryID uint64,
	comment string,
) *boc.Cell {

	c := boc.NewCell()
	forwardTon := big.NewInt(int64(forwardAmount))

	msgBody := abi.JettonTransferMsgBody{
		QueryId:             queryID,
		Amount:              tlb.VarUInteger16(*amount),
		Destination:         destination.ToMsgAddress(),
		ResponseDestination: responseDest.ToMsgAddress(),
		ForwardTonAmount:    tlb.VarUInteger16(*forwardTon),
	}

	if comment != "" {
		msgBody.ForwardPayload.IsRight = true
		// TODO: build Jetton text comment. Use snake ?
		msgBody.ForwardPayload.Value = abi.JettonPayload{
			SumType: abi.TextCommentJettonOp,
			Value:   abi.TextCommentJettonPayload{Text: tlb.Text(comment)},
		}
	}

	if err := c.WriteUint(0xf8a7ea5, 32); err != nil { // transfer#0f8a7ea5
		log.Fatalf("writing Jetton transfer op error")
	}
	if err := tlb.Marshal(c, msgBody); err != nil {
		log.Fatalf("Jetton transfer message serialization error")
	}

	return c
}

func BuildTonWithdrawalMessage(t ExternalWithdrawalTask) *wallet.Message {
	// TODO: check currency?
	return &wallet.Message{
		Amount:  tlb.Coins(t.Amount.BigInt().Uint64()),
		Address: t.Destination.ToAccountID(),
		Body:    buildComment(t.Comment), // TODO: check for empty comment?
		Bounce:  t.Bounceable,
		Mode:    3,
	}
}

func BuildJettonWithdrawalMessage(
	t ExternalWithdrawalTask,
	highloadWallet *wallet.Wallet,
	fromJettonWallet tongo.AccountID,
) *wallet.Message {
	body := MakeJettonTransferMessage(
		t.Destination.ToAccountID(),
		highloadWallet.GetAddress(),
		t.Amount.BigInt(),
		config.JettonForwardAmount,
		uint64(t.QueryID),
		t.Comment,
	)
	return &wallet.Message{
		Amount:  config.JettonTransferTonAmount,
		Address: fromJettonWallet,
		Body:    body,
		Bounce:  true,
		Mode:    3,
	}
}

func BuildJettonProxyWithdrawalMessage(
	proxy JettonProxy,
	jettonWallet, tonWallet tongo.AccountID,
	forwardAmount tlb.Coins,
	amount *big.Int,
	comment string,
) *wallet.Message {
	jettonTransferPayload := MakeJettonTransferMessage(
		tonWallet,
		tonWallet,
		amount,
		forwardAmount,
		uint64(time.Now().UnixNano()), // TODO: check for overflow in DB
		comment,
	)
	msg, err := proxy.BuildPayload(jettonWallet, jettonTransferPayload)
	if err != nil {
		log.Fatalf("build proxy message cell error: %v", err)
	}
	body := boc.NewCell()
	err = body.AddRef(msg)
	if err != nil {
		log.Fatalf("add proxy message as ref error: %v", err)
	}
	return &wallet.Message{
		Amount:  config.JettonTransferTonAmount,
		Address: proxy.Address(),
		Body:    body,
		Bounce:  true,
		Code:    proxy.Code(),
		Data:    proxy.Data(),
		Mode:    3,
	}
}

func buildJettonProxyServiceTonWithdrawalMessage(
	proxy JettonProxy,
	tonWallet tongo.AccountID,
	memo uuid.UUID,
) *wallet.Message {
	msg, err := proxy.BuildPayload(tonWallet, buildComment(memo.String()))
	if err != nil {
		log.Fatalf("build proxy message cell error: %v", err)
	}
	body := boc.NewCell()
	err = body.AddRef(msg)
	if err != nil {
		log.Fatalf("add proxy message as ref error: %v", err)
	}
	return &wallet.Message{
		Amount:  config.JettonTransferTonAmount,
		Address: proxy.Address(),
		Body:    body,
		Bounce:  true,
		Code:    proxy.Code(),
		Data:    proxy.Data(),
		Mode:    3,
	}
}

//func buildTonFillMessage(
//	to tongo.AccountID,
//	amount tlb.Coins,
//	memo uuid.UUID,
//) *wallet.Message {
//	return &wallet.Message{
//		Amount:  amount,
//		Address: to,
//		Body:    buildComment(memo.String()),
//		Bounce:  false,
//		Mode:    3,
//	}
//}
