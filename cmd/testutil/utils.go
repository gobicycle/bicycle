package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"math"
	"sync"
	"time"
)

type PayerProcessor struct {
	client                   *Client
	bcClient                 *blockchain.Connection
	depositsA                []Deposit
	depositsB                []Deposit
	hotWalletsAddrA          map[string]*address.Address
	hotWalletsAddrB          map[string]*address.Address
	lastTxIDA, lastTxIDB     TxID
	balances                 *hotBalances
	knownUUIDsA, knownUUIDsB map[uuid.UUID]struct{}
}

const (
	TonLossForJettonExternalWithdrawal int64 = 58_376_225 // without Jetton wallet deploy except of excess // SCALE:69_579_403 TGR:47_173_046
	TonLossForJettonInternalWithdrawal int64 = 55_306_226 // proxy deploy without Jetton wallet deploy except of JettonForwardAmount // SCALE:57_873_578 TGR:52_738_873
	TonLossForTonExternalWithdrawal    int64 = 10_232_080 // only fees
	TonLossForTonInternalWithdrawal    int64 = 8_034_998  // deposit wallet deploy
)

type PaymentSide string

const (
	SideA PaymentSide = "A"
	SideB PaymentSide = "B"
)

type TxID struct {
	Lt   uint64
	Hash []byte
}

type Deposit struct {
	Address      *address.Address
	JettonWallet *address.Address
	Currency     string
}

type hotBalances struct {
	mutex               sync.Mutex
	hotWalletsBalancesA map[string]int64
	hotWalletsBalancesB map[string]int64
}

func newHotBalances() *hotBalances {
	return &hotBalances{
		hotWalletsBalancesA: make(map[string]int64),
		hotWalletsBalancesB: make(map[string]int64),
	}
}

func (h *hotBalances) ReadBalance(walletSide PaymentSide, currency string) int64 {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	switch walletSide {
	case SideA:
		b, ok := h.hotWalletsBalancesA[currency]
		if ok {
			return b
		}
		return 0
	case SideB:
		b, ok := h.hotWalletsBalancesB[currency]
		if ok {
			return b
		}
		return 0
	}
	log.Fatalf("invalid payment side")
	return 0
}

func (h *hotBalances) WriteBalance(walletSide PaymentSide, currency string, balance int64) {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	switch walletSide {
	case SideA:
		h.hotWalletsBalancesA[currency] = balance
		return
	case SideB:
		h.hotWalletsBalancesB[currency] = balance
		return
	}
	log.Fatalf("invalid payment side")
}

func NewPayerProcessor(
	ctx context.Context,
	client *Client,
	bcClient *blockchain.Connection,
	depositsA map[string][]string,
	depositsB map[string][]string,
	hotA, hotB *address.Address,
) *PayerProcessor {
	// hot jetton wallets
	addrA := make(map[string]*address.Address)
	addrB := make(map[string]*address.Address)
	addrA[core.TonSymbol] = hotA
	addrB[core.TonSymbol] = hotB

	lastTxIDA, err := getLastTxID(ctx, bcClient, hotA)
	if err != nil {
		log.Fatalf("get last TX ID A error: %v", err)
	}
	lastTxIDB, err := getLastTxID(ctx, bcClient, hotB)
	if err != nil {
		log.Fatalf("get last TX ID B error: %v", err)
	}

	for cur, jetton := range config.Config.Jettons {
		jwA, err := bcClient.GetJettonWalletAddress(ctx, hotA, jetton.Master)
		if err != nil {
			log.Fatalf("get hot jetton wallet A error: %v", err)
		}
		jwB, err := bcClient.GetJettonWalletAddress(ctx, hotB, jetton.Master)
		if err != nil {
			log.Fatalf("get hot jetton wallet B error: %v", err)
		}
		addrA[cur] = jwA
		addrB[cur] = jwB
		totalProcessedAmount.WithLabelValues(cur).Set(0)
	}
	totalProcessedAmount.WithLabelValues(core.TonSymbol).Set(0)
	p := &PayerProcessor{
		client:          client,
		bcClient:        bcClient,
		depositsA:       convertDeposits(bcClient, depositsA),
		depositsB:       convertDeposits(bcClient, depositsB),
		hotWalletsAddrA: addrA,
		hotWalletsAddrB: addrB,
		balances:        newHotBalances(),
		lastTxIDA:       lastTxIDA,
		lastTxIDB:       lastTxIDB,
		knownUUIDsA:     make(map[uuid.UUID]struct{}),
		knownUUIDsB:     make(map[uuid.UUID]struct{}),
	}
	return p
}

func convertDeposits(bcClient *blockchain.Connection, deposits map[string][]string) []Deposit {
	var dep []Deposit
	for cur, addresses := range deposits {
		if cur != core.TonSymbol {
			jetton := config.Config.Jettons[cur]
			for _, a := range addresses {
				addr, _ := address.ParseAddr(a)
				jw, err := bcClient.GetJettonWalletAddress(context.Background(), addr, jetton.Master)
				if err != nil {
					log.Fatalf("get jetton wallet error: %v", err)
				}
				dep = append(dep, Deposit{Address: addr, Currency: cur, JettonWallet: jw})
			}
		} else {
			for _, a := range addresses {
				addr, err := address.ParseAddr(a)
				if err != nil {
					log.Fatalf("parse deposit address error: %v", err)
				}
				dep = append(dep, Deposit{Address: addr, Currency: cur})
			}
		}
	}
	return dep
}

func (p *PayerProcessor) Start() {
	go p.balanceMonitor()
	if !onlyMonitoring {
		go p.startPayments(SideA)
		go p.startPayments(SideB)
	}
}

func (p *PayerProcessor) balanceMonitor() {
	log.Infof("Balance monitor started")
	startTotal := make(map[string]int64)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)

		tot, err := p.updateHotWalletBalances(ctx)
		if err != nil {
			log.Errorf("can not update hot wallet balances: %v\n", err)
			cancel()
			continue
		}

		for _, d := range p.depositsA {
			b, err := p.getDepositBalance(ctx, d)
			if err != nil {
				log.Errorf("can not update deposit wallet A balance: %v\n", err)
				cancel()
				continue
			}
			tot[d.Currency] = tot[d.Currency] + b
			depositWalletABalance.WithLabelValues(d.Currency, d.Address.String()).Set(float64(b))
		}

		for _, d := range p.depositsB {
			b, err := p.getDepositBalance(ctx, d)
			if err != nil {
				log.Errorf("can not update deposit wallet B balance: %v\n", err)
				cancel()
				continue
			}
			tot[d.Currency] = tot[d.Currency] + b
			depositWalletBBalance.WithLabelValues(d.Currency, d.Address.String()).Set(float64(b))
		}

		for c, t := range tot {
			totalBalance.WithLabelValues(c).Set(float64(t))
			if _, ok := startTotal[c]; !ok {
				startTotal[c] = t
			}
			totalLosses.WithLabelValues(c).Set(float64(t - startTotal[c]))
		}
		cancel()
		time.Sleep(time.Millisecond * 500)
	}
}

func (p *PayerProcessor) updateHotWalletBalances(ctx context.Context) (map[string]int64, error) {
	totBalance := make(map[string]int64)

	bA, _, err := p.bcClient.GetAccountCurrentState(ctx, p.hotWalletsAddrA[core.TonSymbol])
	if err != nil {
		return nil, err
	}
	bB, _, err := p.bcClient.GetAccountCurrentState(ctx, p.hotWalletsAddrB[core.TonSymbol])
	if err != nil {
		return nil, err
	}

	hotWalletABalance.WithLabelValues(core.TonSymbol).Set(float64(bA.Int64()))
	hotWalletBBalance.WithLabelValues(core.TonSymbol).Set(float64(bB.Int64()))
	totBalance[core.TonSymbol] = bA.Int64() + bB.Int64()
	p.balances.WriteBalance(SideA, core.TonSymbol, bA.Int64())
	p.balances.WriteBalance(SideB, core.TonSymbol, bB.Int64())

	for cur := range config.Config.Jettons {
		bA, err = p.bcClient.GetLastJettonBalance(ctx, p.hotWalletsAddrA[cur])
		if err != nil {
			return nil, err
		}
		bB, err = p.bcClient.GetLastJettonBalance(ctx, p.hotWalletsAddrB[cur])
		if err != nil {
			return nil, err
		}
		hotWalletABalance.WithLabelValues(cur).Set(float64(bA.Int64()))
		hotWalletBBalance.WithLabelValues(cur).Set(float64(bB.Int64()))
		totBalance[cur] = bA.Int64() + bB.Int64()
		p.balances.WriteBalance(SideA, cur, bA.Int64())
		p.balances.WriteBalance(SideB, cur, bB.Int64())
	}
	return totBalance, nil
}

func (p *PayerProcessor) getDepositBalance(ctx context.Context, d Deposit) (int64, error) {
	if d.Currency == core.TonSymbol {
		b, _, err := p.bcClient.GetAccountCurrentState(ctx, d.Address)
		if err != nil {
			return 0, err
		}
		return b.Int64(), nil
	}
	b, err := p.bcClient.GetLastJettonBalance(ctx, d.JettonWallet)
	if err != nil {
		return 0, err
	}
	return b.Int64(), nil
}

func (p *PayerProcessor) startPayments(side PaymentSide) {
	for {
		time.Sleep(time.Second * 60)

		if !p.checkBalances(side) {
			continue
		}

		ids, ww, err := p.withdrawToDeposits(side)
		if err != nil {
			log.Fatalf("withdraw to deposit error: %s", err)
		}
		log.Infof("Withdrawals sended for side %s", side)
		err = p.waitWithdrawals(side, ids)
		if err != nil {
			log.Fatalf("wait withdrawals error: %s", err)
		}

		err = p.validateWithdrawals(context.TODO(), side, ww)
		if err != nil {
			log.Fatalf("validate withdrawals error: %s", err)
		}
		log.Infof("Withdrawals validated for side %s", side)
	}
}

func (p *PayerProcessor) checkBalances(side PaymentSide) bool {
	tonRemained := p.balances.ReadBalance(side, core.TonSymbol) - tonWithdrawAmount*depositsQty
	if tonRemained < tonMinCutoff {
		return false
	}
	for cur := range config.Config.Jettons {
		jettonRemained := p.balances.ReadBalance(side, cur) - jettonWithdrawAmount*depositsQty
		tonRemained = tonRemained - depositsQty*config.JettonTransferTonAmount.NanoTON().Int64()
		if jettonRemained < 0 || tonRemained < tonMinCutoff {
			return false
		}
	}
	return true
}

type withdrawal struct {
	To     *address.Address
	Amount int64
	UUID   uuid.UUID
}

func (p *PayerProcessor) withdrawToDeposits(fromSide PaymentSide) ([]int64, []withdrawal, error) {
	var (
		url      string
		deposits []Deposit
	)
	switch fromSide {
	case SideA:
		deposits = p.depositsB
		url = p.client.urlA
	case SideB:
		deposits = p.depositsA
		url = p.client.urlB
	default:
		return nil, nil, fmt.Errorf("invalid side")
	}
	var (
		ww  []withdrawal
		ids []int64
	)
	for _, d := range deposits {
		if d.Currency == core.TonSymbol {
			r, u, err := p.client.SendWithdrawal(url, d.Currency, d.Address.String(), tonWithdrawAmount)
			if err != nil {
				return nil, nil, err
			}
			ww = append(ww, withdrawal{
				To:     d.Address,
				Amount: tonWithdrawAmount,
				UUID:   u,
			})
			ids = append(ids, r.ID)
			predictedTonLoss.Add(predictLoss(core.TonSymbol))
			totalProcessedAmount.WithLabelValues(core.TonSymbol).Add(float64(tonWithdrawAmount))
		} else {
			r, u, err := p.client.SendWithdrawal(url, d.Currency, d.Address.String(), jettonWithdrawAmount)
			if err != nil {
				return nil, nil, err
			}

			ww = append(ww, withdrawal{
				To:     d.Address,
				Amount: jettonWithdrawAmount,
				UUID:   u,
			})
			ids = append(ids, r.ID)
			predictedTonLoss.Add(predictLoss(d.Currency))
			totalProcessedAmount.WithLabelValues(d.Currency).Add(float64(jettonWithdrawAmount))
		}
	}
	return ids, ww, nil
}

func (p *PayerProcessor) waitWithdrawals(fromSide PaymentSide, ids []int64) error {
	var url string
	switch fromSide {
	case SideA:
		url = p.client.urlA
	case SideB:
		url = p.client.urlB
	default:
		return fmt.Errorf("invalid side")
	}
	completed := make(map[int64]struct{})
	for {
		for _, id := range ids {
			_, ok := completed[id]
			if ok {
				continue
			}
			r, err := p.client.GetWithdrawalStatus(url, id)
			if err != nil {
				return err
			}
			if r.Status == core.ProcessedStatus {
				completed[id] = struct{}{}
			} else {
				time.Sleep(time.Millisecond * 200)
				break
			}
		}
		if len(completed) == len(ids) {
			break
		}
	}
	return nil
}

func (p *PayerProcessor) validateWithdrawals(ctx context.Context, fromSide PaymentSide, withdrawals []withdrawal) error {
	var (
		addr     *address.Address
		lastTxID TxID
		newUUIDs []uuid.UUID
	)
	switch fromSide {
	case SideA:
		addr = p.hotWalletsAddrA[core.TonSymbol]
		lastTxID = p.lastTxIDA
	case SideB:
		addr = p.hotWalletsAddrB[core.TonSymbol]
		lastTxID = p.lastTxIDB
	default:
		return fmt.Errorf("invalid side")
	}

	txs, err := p.loadTXs(ctx, lastTxID, addr)
	if err != nil {
		return err
	}
	if len(txs) > 0 {
		lastTxID = TxID{
			Lt:   txs[0].LT,
			Hash: txs[0].Hash,
		}
	}

	remainingTXs := withdrawals
	for _, tx := range txs {
		ww, uuids, err := parseTX(tx)
		if err != nil {
			return err
		}
		newUUIDs = append(newUUIDs, uuids...)
		remainingTXs = compareWithdrawals(ww, remainingTXs)
	}
	if len(remainingTXs) > 0 {
		var s string
		for _, r := range remainingTXs {
			s = s + r.UUID.String() + ", "
		}
		return fmt.Errorf("can not find withdrawals: %s", s)
	}

	switch fromSide {
	case SideA:
		p.lastTxIDA = lastTxID
		err := checkDoubleSpending(newUUIDs, p.knownUUIDsA)
		if err != nil {
			return err
		}
	case SideB:
		p.lastTxIDB = lastTxID
		err := checkDoubleSpending(newUUIDs, p.knownUUIDsB)
		if err != nil {
			return err
		}
	}
	return nil
}

func checkDoubleSpending(newUUIDs []uuid.UUID, knownUUIDs map[uuid.UUID]struct{}) error {
	for _, u := range newUUIDs {
		_, ok := knownUUIDs[u]
		if ok {
			return fmt.Errorf("double spending: %s", u.String())
		}
		knownUUIDs[u] = struct{}{}
	}
	return nil
}

func (p *PayerProcessor) loadTXs(ctx context.Context, lastTxID TxID, addr *address.Address) ([]*tlb.Transaction, error) {
	newTxID, err := getLastTxID(ctx, p.bcClient, addr)
	if err != nil {
		return nil, err
	}

	currentTxID := newTxID
	txs := make([]*tlb.Transaction, 0)

	for {
		// last transaction has 0 prev lt
		if currentTxID.Lt == 0 {
			break
		}

		list, err := p.bcClient.ListTransactions(ctx, addr, 3, currentTxID.Lt, currentTxID.Hash)
		if err != nil {
			return nil, err
		}

		// oldest = first in list
		for i := len(list) - 1; i >= 0; i-- {
			if bytes.Equal(list[i].Hash, lastTxID.Hash) {
				return txs, nil
			}
			txs = append(txs, list[i])
		}

		// set previous info from the oldest transaction in list
		currentTxID.Hash = list[0].PrevTxHash
		currentTxID.Lt = list[0].PrevTxLT
	}
	return nil, fmt.Errorf("can not get txs")
}

func parseTX(tx *tlb.Transaction) ([]withdrawal, []uuid.UUID, error) {
	var (
		ww    []withdrawal
		uuids []uuid.UUID
	)
	outMessages, err := tx.IO.Out.ToSlice()
	if err != nil {
		return nil, nil, err
	}
	for _, m := range outMessages {
		if m.MsgType != tlb.MsgTypeInternal {
			continue
		}
		msg := m.AsInternal()
		jt, err := core.DecodeJettonTransfer(msg)
		if err == nil {
			u, err := uuid.FromString(jt.Comment)
			if err != nil {
				return nil, nil, err
			}
			ww = append(ww, withdrawal{
				To:     jt.Destination,
				Amount: jt.Amount.BigInt().Int64(),
				UUID:   u,
			})
			uuids = append(uuids, u)
		} else if msg.Body.BitsSize() > 32 {
			u, err := uuid.FromString(core.LoadComment(msg.Body))
			if err != nil {
				return nil, nil, err
			}
			ww = append(ww, withdrawal{
				To:     msg.DstAddr,
				Amount: msg.Amount.NanoTON().Int64(),
				UUID:   u,
			})
			uuids = append(uuids, u)
		}
	}
	return ww, uuids, nil
}

func compareWithdrawals(all, target []withdrawal) []withdrawal {
	var res []withdrawal
	for _, t := range target {
		found := false
		for _, a := range all {
			if t.UUID == a.UUID {
				found = true
				break
			}
		}
		if !found {
			res = append(res, t)
		}
	}
	return res
}

func getLastTxID(ctx context.Context, bcClient *blockchain.Connection, address *address.Address) (TxID, error) {
	b, err := bcClient.GetMasterchainInfo(ctx)
	if err != nil {
		return TxID{}, err
	}
	res, err := bcClient.GetAccount(ctx, b, address)
	if err != nil {
		return TxID{}, err
	}
	return TxID{
		Lt:   res.LastTxLT,
		Hash: res.LastTxHash,
	}, nil
}

func predictLoss(currency string) float64 {
	if currency == core.TonSymbol {
		cutoff := config.Config.Ton.Withdrawal.Int64()
		n := math.Ceil(float64(cutoff) / float64(tonWithdrawAmount)) // number of replenishments of the deposit before withdrawal
		return float64(TonLossForTonExternalWithdrawal) + float64(TonLossForTonInternalWithdrawal)/n
	} else {
		cutoff := config.Config.Jettons[currency].WithdrawalCutoff.Int64()
		n := math.Ceil(float64(cutoff) / float64(jettonWithdrawAmount)) // number of replenishments of the deposit before withdrawal
		return float64(TonLossForJettonExternalWithdrawal) + float64(TonLossForJettonInternalWithdrawal)/n
	}
}
