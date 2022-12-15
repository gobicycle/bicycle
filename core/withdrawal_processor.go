package core

import (
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/audit"
	"github.com/gobicycle/bicycle/config"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
)

type WithdrawalsProcessor struct {
	db               storage
	bc               blockchain
	wallets          Wallets
	coldWallet       *address.Address
	wg               *sync.WaitGroup
	gracefulShutdown atomic.Bool
}

type internalWithdrawal struct {
	Memo uuid.UUID
	Task InternalWithdrawalTask
}

type withdrawals struct {
	Messages []*wallet.Message
	External []ExternalWithdrawalTask
	Internal []internalWithdrawal
	Service  []ServiceWithdrawalTask
}

func NewWithdrawalsProcessor(
	wg *sync.WaitGroup,
	db storage,
	bc blockchain,
	wallets Wallets,
	coldWallet *address.Address,
) *WithdrawalsProcessor {
	w := &WithdrawalsProcessor{
		db:         db,
		bc:         bc,
		wallets:    wallets,
		coldWallet: coldWallet,
		wg:         wg,
	}
	return w
}

func (p *WithdrawalsProcessor) Start() {
	p.wg.Add(3)
	go p.startWithdrawalsProcessor()
	go p.startInternalTonWithdrawalsProcessor()
	go p.startExpirationProcessor()
}

func (p *WithdrawalsProcessor) Stop() {
	p.gracefulShutdown.Store(true)
}

func (p *WithdrawalsProcessor) startWithdrawalsProcessor() {
	defer p.wg.Done()
	log.Infof("ExternalWithdrawalsProcessor started")
	for {
		p.waitSync() // gracefulShutdown break must be after waitSync
		if p.gracefulShutdown.Load() {
			log.Infof("ExternalWithdrawalsProcessor stopped")
			break
		}
		time.Sleep(config.ExternalWithdrawalPeriod)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*15) // must be < ExternalWithdrawalPeriod
		defer cancel()
		err := p.makeColdWalletWithdrawals(ctx)
		if err != nil {
			log.Fatalf("make withdrawals to cold wallet error: %v\n", err)
		}
		w, err := p.buildWithdrawalMessages(ctx)
		if err != nil {
			log.Fatalf("make withdrawal messages error: %v\n", err)
		}
		if len(w.Messages) == 0 {
			continue
		}
		extMsg, err := p.wallets.TonHotWallet.BuildMessageForMany(ctx, w.Messages)
		if err != nil {
			log.Fatalf("build hotwallet external msg error: %v\n", err)
		}
		info, err := getHighLoadWalletExtMsgInfo(extMsg)
		if err != nil {
			log.Fatalf("get external message uuid error: %v\n", err)
		}
		err = p.db.CreateExternalWithdrawals(ctx, w.External, info.UUID, info.TTL)
		if err != nil {
			log.Fatalf("save external withdrawals error: %v\n", err)
		}
		for _, sw := range w.Service {
			err = p.db.UpdateServiceWithdrawalRequest(ctx, sw, info.TTL)
			if err != nil {
				log.Fatalf("update service withdrawal error: %v\n", err)
			}
		}
		for _, iw := range w.Internal {
			err = p.db.SaveInternalWithdrawalTask(ctx, iw.Task, info.TTL, iw.Memo)
			if err != nil {
				log.Fatalf("save internal withdrawal error: %v\n", err)
			}
		}
		err = p.bc.SendExternalMessage(ctx, extMsg)
		if err != nil {
			log.Errorf("send external msg error: %v\n", err)
		}
	}
}

func (p *WithdrawalsProcessor) buildWithdrawalMessages(ctx context.Context) (withdrawals, error) {
	var (
		usedAddresses []Address
		res           withdrawals
	)

	balances, err := p.getHotWalletBalances(ctx)
	if err != nil {
		return withdrawals{}, err
	}

	// TODO: refactor to new API methods
	serviceTasks, err := p.db.GetServiceWithdrawalTasks(ctx, 250)
	if err != nil {
		return withdrawals{}, err
	}
	for _, t := range serviceTasks {
		if decreaseBalances(balances, TonSymbol, config.JettonTransferTonAmount.NanoTON()) {
			continue
		}
		msg, err := p.buildServiceWithdrawalMessage(ctx, t)
		if err != nil {
			return withdrawals{}, err
		}
		usedAddresses = append(usedAddresses, t.From)
		res.Messages = append(res.Messages, msg...)
		res.Service = append(res.Service, t)
	}

	internalTasks, err := p.db.GetJettonInternalWithdrawalTasks(ctx, usedAddresses, 250)
	if err != nil {
		return withdrawals{}, err
	}
	for _, t := range internalTasks {
		if len(res.Messages) > 254 {
			break
		}
		if decreaseBalances(balances, TonSymbol, config.JettonTransferTonAmount.NanoTON()) {
			continue
		}
		msg, memo, err := p.buildJettonInternalWithdrawalMessage(ctx, t)
		if err != nil {
			return withdrawals{}, err
		}
		if len(msg) != 0 {
			res.Messages = append(res.Messages, msg...)
			res.Internal = append(res.Internal, internalWithdrawal{
				Task: t,
				Memo: memo,
			})
		}
	}

	// not filter usedAddresses by DB and perform internal addresses checking and logging
	externalTasks, err := p.db.GetExternalWithdrawalTasks(ctx, 250)
	if err != nil {
		return withdrawals{}, err
	}
	for _, w := range externalTasks {
		if len(res.Messages) > 254 {
			break
		}
		t, ok := p.db.GetWalletType(w.Destination)
		if ok {
			audit.Log(audit.Warning, string(TonHotWallet),
				fmt.Sprintf("Withdrawal task to internal %s address: %s", t, w.Destination.ToUserFormat()))
			continue
		}
		if decreaseBalances(balances, w.Currency, w.Amount.BigInt()) {
			continue
		}
		msg := p.buildExternalWithdrawalMessage(w)
		// TODO: check tons amount for jetton wallet deploy
		res.Messages = append(res.Messages, msg)
		res.External = append(res.External, w)
	}
	return res, nil
}

func (p *WithdrawalsProcessor) getHotWalletBalances(ctx context.Context) (map[string]*big.Int, error) {
	res := make(map[string]*big.Int)
	balance, _, err := p.bc.GetAccountCurrentState(ctx, p.wallets.TonHotWallet.Address())
	if err != nil {
		return nil, err
	}
	res[TonSymbol] = balance
	for cur, w := range p.wallets.JettonHotWallets {
		balance, err := p.bc.GetLastJettonBalance(ctx, w.Address)
		if err != nil {
			return nil, err
		}
		res[cur] = balance
	}
	return res, nil
}

// decreaseBalances returns true if balance < amount
func decreaseBalances(balances map[string]*big.Int, currency string, amount *big.Int) bool {
	if currency == TonSymbol {
		if balances[TonSymbol].Cmp(amount) == -1 { // balance < amount
			return true
		}
		balances[TonSymbol].Sub(balances[TonSymbol], amount)
		return false
	}
	if balances[currency].Cmp(amount) == -1 || // balance < amount
		balances[TonSymbol].Cmp(config.JettonTransferTonAmount.NanoTON()) == -1 { // balance < JettonTransferTonAmount
		return true
	}
	balances[currency].Sub(balances[currency], amount)
	balances[TonSymbol].Sub(balances[TonSymbol], config.JettonTransferTonAmount.NanoTON())
	return false
}

func (p *WithdrawalsProcessor) buildJettonInternalWithdrawalMessage(
	ctx context.Context,
	task InternalWithdrawalTask,
) (
	[]*wallet.Message,
	uuid.UUID,
	error,
) {
	proxy, err := NewJettonProxy(task.SubwalletID, p.wallets.TonHotWallet.Address())
	if err != nil {
		return nil, uuid.UUID{}, err
	}
	jettonWalletAddress := task.From.ToTonutilsAddressStd(0)
	balance, err := p.bc.GetLastJettonBalance(ctx, jettonWalletAddress)
	if err != nil {
		return nil, uuid.UUID{}, err
	}
	if balance.Cmp(config.Config.Jettons[task.Currency].WithdrawalCutoff) == 1 { // balance > MinimalJettonWithdrawalAmount
		memo, err := uuid.NewV4()
		if err != nil {
			return nil, uuid.UUID{}, err
		}
		msg := BuildJettonProxyWithdrawalMessage(
			*proxy,
			jettonWalletAddress,
			p.wallets.TonHotWallet.Address(),
			balance,
			config.JettonForwardAmount,
			memo.String(),
		)
		return []*wallet.Message{msg}, memo, nil
	}
	return []*wallet.Message{}, uuid.UUID{}, nil
}

func (p *WithdrawalsProcessor) buildServiceWithdrawalMessage(
	ctx context.Context,
	task ServiceWithdrawalTask,
) (
	[]*wallet.Message,
	error,
) {
	proxy, err := NewJettonProxy(task.SubwalletID, p.wallets.TonHotWallet.Address())
	if err != nil {
		return nil, err
	}

	if task.JettonMaster == nil { // full TON withdrawal from Jetton proxy
		msg := buildJettonProxyServiceTonWithdrawalMessage(*proxy, p.wallets.TonHotWallet.Address(), task.Memo)
		return []*wallet.Message{msg}, nil
	}

	// TON or Jetton withdrawal from Jetton wallet
	jettonWallet, err := p.bc.GetJettonWalletAddress(ctx, proxy.address, task.JettonMaster.ToTonutilsAddressStd(0))
	if err != nil {
		return nil, err
	}
	t, ok := p.db.GetWalletTypeByTonutilsAddress(jettonWallet)
	if ok && t != JettonDepositWallet {
		audit.Log(audit.Warning, string(JettonOwner),
			fmt.Sprintf("Attempt of service withdrawal from internal not Jetton deposit address: %s",
				jettonWallet.String()))
		return []*wallet.Message{}, nil
	}
	if ok && t == JettonDepositWallet && task.JettonAmount.Cmp(ZeroCoins()) == 1 {
		audit.Log(audit.Warning, string(JettonOwner),
			fmt.Sprintf("Attempt of service Jetton withdrawal from Jetton deposit (only TONs allowed). Address: %s",
				jettonWallet.String()))
		return []*wallet.Message{}, nil
	}
	jettonBalance, err := p.bc.GetLastJettonBalance(ctx, jettonWallet)
	if err != nil {
		return nil, err
	}
	if jettonBalance.Cmp(task.JettonAmount.BigInt()) == -1 { // jettonBalance < JettonAmount
		audit.Log(audit.Warning, string(JettonOwner),
			fmt.Sprintf("Unsufficient Jetton amount %s (%s needed) on wallet %s",
				jettonBalance.String(), task.JettonAmount.String(), jettonWallet.String()))
		return []*wallet.Message{}, nil
	}

	tonBalance, _, err := p.bc.GetAccountCurrentState(ctx, jettonWallet)
	if err != nil {
		return nil, err
	}
	if tonBalance.Cmp(task.TonAmount.BigInt()) == -1 { // tonBalance < TonAmount
		audit.Log(audit.Warning, string(JettonOwner),
			fmt.Sprintf("Unsufficient TON amount %s (%s needed) on wallet %s",
				tonBalance.String(), task.TonAmount.String(), jettonWallet.String()))
		return []*wallet.Message{}, nil
	}

	msg := BuildJettonProxyWithdrawalMessage(
		*proxy,
		jettonWallet,
		p.wallets.TonHotWallet.Address(),
		task.JettonAmount.BigInt(),
		tlb.FromNanoTON(task.TonAmount.BigInt()),
		task.Memo.String(),
	)
	return []*wallet.Message{msg}, nil
}

func (p *WithdrawalsProcessor) buildExternalWithdrawalMessage(wt ExternalWithdrawalTask) *wallet.Message {
	if wt.Currency == TonSymbol {
		return BuildTonWithdrawalMessage(wt)
	}
	jw := p.wallets.JettonHotWallets[wt.Currency]
	return BuildJettonWithdrawalMessage(wt, p.wallets.TonHotWallet, jw.Address)
}

func (p *WithdrawalsProcessor) startExpirationProcessor() {
	log.Infof("ExpirationProcessor started")
	defer p.wg.Done()
	for {
		p.waitSync() // gracefulShutdown break must be after waitSync
		if p.gracefulShutdown.Load() {
			log.Infof("ExpirationProcessor stopped")
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // must be < ExpirationProcessorPeriod
		defer cancel()
		err := p.db.SetExpired(ctx)
		if err != nil {
			log.Fatalf("set expired withdrawals error: %v", err)
		}
		time.Sleep(config.ExpirationProcessorPeriod)
	}
}

func (p *WithdrawalsProcessor) startInternalTonWithdrawalsProcessor() {
	defer p.wg.Done()
	log.Infof("InternalTonWithdrawalsProcessor started")
	for {
		p.waitSync() // gracefulShutdown break must be after waitSync
		if p.gracefulShutdown.Load() {
			log.Infof("InternalTonWithdrawalsProcessor stopped")
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*3) // must be < InternalWithdrawalPeriod
		defer cancel()
		tasks, err := p.db.GetTonInternalWithdrawalTasks(ctx, 40) // context limitation
		if err != nil {
			log.Fatalf("get internal withdrawal tasks error: %v", err)
		}
		for _, task := range tasks {
			err = p.withdrawTONsFromDeposit(ctx, task)
			if err != nil {
				log.Fatalf("TONs internal withdrawal error: %v", err)
			}
			time.Sleep(time.Millisecond * 50)
		}
		time.Sleep(config.InternalWithdrawalPeriod)
	}
}

func (p *WithdrawalsProcessor) withdrawTONsFromDeposit(ctx context.Context, task InternalWithdrawalTask) error {
	subwallet, err := p.wallets.TonBasicWallet.GetSubwallet(task.SubwalletID)
	if err != nil {
		return err
	}
	spec := subwallet.GetSpec().(*wallet.SpecV3)
	spec.SetMessagesTTL(uint32(config.ExternalMessageLifetime.Seconds()))

	balance, state, err := p.bc.GetAccountCurrentState(ctx, subwallet.Address())
	if err != nil {
		return err
	}
	if state == tlb.AccountStatusNonExist {
		return nil
	}
	if balance.Cmp(config.Config.Ton.Withdrawal) == 1 { // Balance > MinimalTonWithdrawalAmount
		memo, err := uuid.NewV4()
		if err != nil {
			return err
		}
		err = p.db.SaveInternalWithdrawalTask(ctx, task, time.Now().Add(config.ExternalMessageLifetime), memo)
		if err != nil {
			return err
		}
		// time.Now().Add(config.ExternalMessageLifetime) and real TTL
		// should be very close since the withdrawal occurs immediately
		err = WithdrawTONs(ctx, subwallet, p.wallets.TonHotWallet, memo.String())
		if err != nil {
			log.Errorf("TONs internal withdrawal error: %v", err)
			return nil
		}
	}
	return nil
}

func (p *WithdrawalsProcessor) waitSync() {
	for {
		if p.gracefulShutdown.Load() {
			log.Infof("WaitSync interrupted")
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)
		defer cancel()
		isSynced, err := p.db.IsActualBlockData(ctx)
		if err != nil {
			log.Fatalf("check sync error: %v", err)
		}
		if isSynced {
			break
		}
		time.Sleep(time.Second * 3)
	}
}

func (p *WithdrawalsProcessor) makeColdWalletWithdrawals(ctx context.Context) error {
	if p.coldWallet == nil {
		return nil
	}

	tonBalance, _, err := p.bc.GetAccountCurrentState(ctx, p.wallets.TonHotWallet.Address())
	if err != nil {
		return err
	}
	dest := AddressMustFromTonutilsAddress(p.coldWallet)

	for cur, jw := range p.wallets.JettonHotWallets {
		inProgress, err := p.db.IsInProgressInternalWithdrawalRequest(ctx, dest, cur)
		if err != nil {
			return err
		}
		if inProgress {
			continue
		}
		jettonBalance, err := p.bc.GetLastJettonBalance(ctx, jw.Address)
		if err != nil {
			return err
		}
		if jettonBalance.Cmp(config.Config.Jettons[cur].HotWalletMaxCutoff) != 1 { // jettonBalance <= HotWalletMaxCutoff
			continue
		}
		jettonAmount := big.NewInt(0)
		u, err := uuid.NewV4()
		if err != nil {
			return err
		}
		jettonAmount.Sub(jettonBalance, config.Config.Jettons[cur].HotWalletMaxCutoff)
		tonBalance.Sub(tonBalance, config.JettonTransferTonAmount.NanoTON())
		req := WithdrawalRequest{
			Currency:    jw.Currency,
			Amount:      NewCoins(jettonAmount),
			Bounceable:  true,
			Destination: dest,
			IsInternal:  true,
			QueryID:     u.String(),
		}
		_, err = p.db.SaveWithdrawalRequest(ctx, req)
		if err != nil {
			return err
		}
		log.Infof("%v withdrawal to cold wallet saved", cur)
	}

	inProgress, err := p.db.IsInProgressInternalWithdrawalRequest(ctx, dest, TonSymbol)
	if err != nil {
		return err
	}
	if inProgress {
		return nil
	}

	if tonBalance.Cmp(config.Config.Ton.HotWalletMax) != 1 { // tonBalance <= HotWalletMax
		return nil
	}

	tonAmount := big.NewInt(0)
	u, err := uuid.NewV4()
	if err != nil {
		return err
	}
	tonAmount.Sub(tonBalance, config.Config.Ton.HotWalletMax)
	req := WithdrawalRequest{
		Currency:    TonSymbol,
		Amount:      NewCoins(tonAmount),
		Bounceable:  p.coldWallet.IsBounceable(),
		Destination: dest,
		IsInternal:  true,
		QueryID:     u.String(),
	}

	_, err = p.db.SaveWithdrawalRequest(ctx, req)
	if err != nil {
		return err
	}
	log.Infof("TON withdrawal to cold wallet saved")
	return nil
}
