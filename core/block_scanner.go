package core

import (
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/audit"
	"github.com/gobicycle/bicycle/config"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/startfellows/tongo"
	"github.com/startfellows/tongo/boc"
	tongoTlb "github.com/startfellows/tongo/tlb"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/tvm/cell"
	"math/big"
	"sync"
	"time"
)

type BlockScanner struct {
	db         storage
	blockchain blockchain
	shard      byte
	tracker    blocksTracker
	wg         *sync.WaitGroup
	queue      queue
}

type transactions struct {
	Address      Address
	WalletType   WalletType
	Transactions []*tlb.Transaction
}

type jettonTransferNotificationMsg struct {
	Amount  Coins
	Sender  *address.Address
	Comment string
}

type JettonTransferMsg struct {
	Amount      Coins
	Destination *address.Address
	Comment     string
}

type HighLoadWalletExtMsgInfo struct {
	UUID     uuid.UUID
	TTL      time.Time
	Messages *cell.Dictionary
}

type incomeNotification struct {
	Deposit   string `json:"deposit"`
	Amount    string `json:"amount"`
	Timestamp int64  `json:"timestamp"`
}

func NewBlockScanner(
	wg *sync.WaitGroup,
	db storage,
	blockchain blockchain,
	shard byte,
	tracker blocksTracker,
	queueClient queue,
) *BlockScanner {
	t := &BlockScanner{
		db:         db,
		blockchain: blockchain,
		shard:      shard,
		tracker:    tracker,
		wg:         wg,
		queue:      queueClient,
	}
	t.wg.Add(1)
	go t.Start()
	return t
}

func (s *BlockScanner) Start() {
	defer s.wg.Done()
	log.Printf("Block scanner started")
	for {
		block, exit, err := s.tracker.NextBlock()
		if err != nil {
			log.Fatalf("get block error: %v", err)
		}
		if exit {
			log.Printf("Block scanner stopped")
			break
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		err = s.processBlock(ctx, block)
		if err != nil {
			log.Fatalf("block processing error: %v", err)
		}
		cancel()
	}
}

func (s *BlockScanner) Stop() {
	s.tracker.Stop()
}

func (s *BlockScanner) processBlock(ctx context.Context, block ShardBlockHeader) error {
	txIDs, err := s.blockchain.GetTransactionIDsFromBlock(ctx, block.BlockInfo)
	if err != nil {
		return err
	}
	filteredTXs, err := s.filterTXs(ctx, block.BlockInfo, txIDs)
	if err != nil {
		return err
	}
	e, err := s.processTXs(ctx, filteredTXs, block)
	if err != nil {
		return err
	}
	err = s.pushNotifications(e)
	if err != nil {
		return err
	}
	return s.db.SaveParsedBlockData(ctx, e)
}

func (s *BlockScanner) pushNotifications(e BlockEvents) error {
	if !config.Config.QueueEnabled {
		return nil
	}
	if config.Config.DepositSideBalances {
		for _, ei := range e.ExternalIncomes {
			err := s.pushNotification(ei.To, ei.Amount, ei.Utime)
			if err != nil {
				return err
			}
		}
	} else {
		for _, ii := range e.InternalIncomes {
			err := s.pushNotification(ii.From, ii.Amount, ii.Utime)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *BlockScanner) pushNotification(addr Address, amount Coins, timestamp uint32) error {
	owner := s.db.GetOwner(addr)
	if owner != nil {
		addr = *owner
	}
	notification := incomeNotification{
		Deposit:   addr.ToUserFormat(),
		Amount:    amount.String(),
		Timestamp: int64(timestamp),
	}
	return s.queue.Publish(notification)
}

func (s *BlockScanner) filterTXs(
	ctx context.Context,
	blockID *tlb.BlockInfo,
	ids []*tlb.TransactionID,
) (
	[]transactions, error,
) {
	txMap := make(map[Address][]*tlb.Transaction)
	for _, id := range ids {
		a, err := AddressFromBytes(id.AccountID) // must be int256 for lite api
		if err != nil {
			return nil, err
		}
		_, ok := s.db.GetWalletType(a)
		if ok {
			tx, err := s.blockchain.GetTransactionFromBlock(ctx, blockID, id)
			if err != nil {
				return nil, err
			}
			txMap[a] = append(txMap[a], tx)
		}
	}
	var res []transactions
	for a, txs := range txMap {
		wType, _ := s.db.GetWalletType(a)
		res = append(res, transactions{a, wType, txs})
	}
	return res, nil
}

func checkTxForSuccess(tx *tlb.Transaction) (bool, error) {
	descData := tx.Description.ToBOC()
	c, err := boc.DeserializeBoc(descData)
	if err != nil {
		return false, err
	}
	var desc tongo.TransactionDescr
	err = tongoTlb.Unmarshal(c[0], &desc)
	if err != nil {
		return false, err
	}
	var fakeTx tongo.Transaction // need for check tx success via tongo
	fakeTx.Description = desc
	return fakeTx.IsSuccess(), nil
}

func (s *BlockScanner) processTXs(
	ctx context.Context,
	txs []transactions,
	block ShardBlockHeader,
) (
	BlockEvents, error,
) {
	blockEvents := BlockEvents{Block: block}
	for _, t := range txs {
		switch t.WalletType {
		case TonHotWallet:
			hotWalletEvents, err := s.processTonHotWalletTXs(t)
			if err != nil {
				return BlockEvents{}, err
			}
			blockEvents.Append(hotWalletEvents)
		case TonDepositWallet:
			tonDepositEvents, err := s.processTonDepositWalletTXs(t)
			if err != nil {
				return BlockEvents{}, err
			}
			blockEvents.Append(tonDepositEvents)
		case JettonDepositWallet:
			jettonDepositEvents, err := s.processJettonDepositWalletTXs(ctx, t, block.BlockInfo, block.Parent)
			if err != nil {
				return BlockEvents{}, err
			}
			blockEvents.Append(jettonDepositEvents)
		}
	}
	return blockEvents, nil
}

func (s *BlockScanner) processTonHotWalletTXs(txs transactions) (Events, error) {
	var events Events

	for _, tx := range txs.Transactions {

		if tx.IO.In == nil { // impossible for standard highload TON wallet
			audit.LogTX(audit.Error, string(TonHotWallet), tx.Hash, "transaction without in message")
			return Events{}, fmt.Errorf("anomalous behavior of the TON hot wallet")
		}

		switch tx.IO.In.MsgType {
		case tlb.MsgTypeExternalIn:
			e, err := s.processTonHotWalletExternalInMsg(tx)
			if err != nil {
				return Events{}, err
			}
			events.Append(e)
		case tlb.MsgTypeInternal:
			e, err := s.processTonHotWalletInternalInMsg(tx)
			if err != nil {
				return Events{}, err
			}
			events.Append(e)
		default:
			audit.LogTX(audit.Error, string(TonHotWallet), tx.Hash,
				"transaction in message must be internal or external in")
			return Events{}, fmt.Errorf("anomalous behavior of the TON hot wallet")
		}
	}
	return events, nil
}

func (s *BlockScanner) processTonDepositWalletTXs(txs transactions) (Events, error) {
	var events Events

	for _, tx := range txs.Transactions {

		if tx.IO.In == nil { // impossible for standard TON V3 wallet
			audit.LogTX(audit.Error, string(TonDepositWallet), tx.Hash, "transaction without in message")
			return Events{}, fmt.Errorf("anomalous behavior of the deposit TON wallet")
		}

		success, err := checkTxForSuccess(tx)
		if err != nil {
			return Events{}, err
		}
		if !success {
			audit.LogTX(audit.Info, string(TonDepositWallet), tx.Hash, "failed transaction")
			continue
		}

		switch tx.IO.In.MsgType {
		case tlb.MsgTypeExternalIn:
			// internal withdrawal. spam or invalid external cannot invoke tx
			// theoretically will be up to 4 out messages for TON V3 wallet
			// external_in msg without out_msg very rare or impossible
			// it is not critical for internal transfers (double spending not dangerous).
			e, err := s.processTonDepositWalletExternalInMsg(tx)
			if err != nil {
				return Events{}, err
			}
			events.Append(e)
		case tlb.MsgTypeInternal:
			// success external income (without bounce)
			// internal message can not invoke out message for TON wallet V3 except of bounce
			// bounced filtered at !success step
			if len(tx.IO.Out) != 0 {
				audit.LogTX(audit.Error, string(TonDepositWallet), tx.Hash, "outgoing message from internal incoming")
				return Events{}, fmt.Errorf("anomalous behavior of the deposit TON wallet")
			}
			e, err := s.processTonDepositWalletInternalInMsg(tx)
			if err != nil {
				return Events{}, err
			}
			events.Append(e)
		default:
			audit.LogTX(audit.Error, string(TonDepositWallet), tx.Hash,
				"transaction in message must be internal or external in")
			return Events{}, fmt.Errorf("anomalous behavior of the deposit TON wallet")
		}
	}
	return events, nil
}

func (s *BlockScanner) processJettonDepositWalletTXs(
	ctx context.Context,
	txs transactions,
	blockID, prevBlockID *tlb.BlockInfo,
) (Events, error) {
	var (
		unknownTransactions []*tlb.Transaction
		events              Events
	)

	knownIncomeAmount := big.NewInt(0)
	totalWithdrawalsAmount := big.NewInt(0)

	for _, tx := range txs.Transactions {
		e, knownAmount, outUnknownFound, err := s.processJettonDepositOutMsgs(tx)
		if err != nil {
			return Events{}, err
		}
		knownIncomeAmount.Add(knownIncomeAmount, knownAmount)
		events.Append(e)

		e, totalAmount, inUnknownFound, err := s.processJettonDepositInMsg(tx)
		if err != nil {
			return Events{}, err
		}
		totalWithdrawalsAmount.Add(totalWithdrawalsAmount, totalAmount)
		events.Append(e)

		if outUnknownFound || inUnknownFound { // if found some unknown messages that potentially can change Jetton balance
			unknownTransactions = append(unknownTransactions, tx)
		}
	}

	unknownIncomeAmount, err := s.calculateJettonAmounts(ctx, txs.Address, prevBlockID, blockID, knownIncomeAmount, totalWithdrawalsAmount)
	if err != nil {
		return Events{}, err
	}

	if unknownIncomeAmount.Cmp(big.NewInt(0)) == 1 { // unknownIncomeAmount > 0
		unknownIncomes, err := convertUnknownJettonTxs(unknownTransactions, txs.Address, unknownIncomeAmount)
		if err != nil {
			return Events{}, err
		}
		events.ExternalIncomes = append(events.ExternalIncomes, unknownIncomes...)
	}

	return events, nil
}

func (s *BlockScanner) calculateJettonAmounts(
	ctx context.Context,
	address Address,
	prevBlockID, blockID *tlb.BlockInfo,
	knownIncomeAmount, totalWithdrawalsAmount *big.Int,
) (
	unknownIncomeAmount *big.Int,
	err error,
) {
	prevBalance, err := s.blockchain.GetJettonBalance(ctx, address, prevBlockID)
	if err != nil {
		return nil, err
	}
	currentBalance, err := s.blockchain.GetJettonBalance(ctx, address, blockID)
	if err != nil {
		return nil, err
	}
	diff := big.NewInt(0)
	diff.Sub(currentBalance, prevBalance) // diff = currentBalance - prevBalance

	totalIncomeAmount := big.NewInt(0)
	totalIncomeAmount.Add(diff, totalWithdrawalsAmount) // totalIncomeAmount = diff + totalWithdrawalsAmount

	unknownIncomeAmount = big.NewInt(0)
	unknownIncomeAmount.Sub(totalIncomeAmount, knownIncomeAmount) // unknownIncomeAmount = totalIncomeAmount - knownIncomeAmount

	return unknownIncomeAmount, nil
}

func convertUnknownJettonTxs(txs []*tlb.Transaction, addr Address, amount *big.Int) ([]ExternalIncome, error) {
	var incomes []ExternalIncome
	for _, tx := range txs {
		if tx.IO.In == nil || tx.IO.In.MsgType != tlb.MsgTypeInternal { // unknown sender. do not process MsgAddressExt
			incomes = append(incomes, ExternalIncome{
				Utime:  tx.Now,
				Lt:     tx.LT,
				To:     addr,
				Amount: ZeroCoins(),
			})
		} else {
			inMsg := tx.IO.In.AsInternal()
			var from []byte
			if inMsg.SrcAddr != nil && inMsg.SrcAddr.Type() == address.StdAddress {
				from = inMsg.SrcAddr.Data()
			}
			incomes = append(incomes, ExternalIncome{
				Utime:  tx.Now,
				Lt:     tx.LT,
				From:   from,
				To:     addr,
				Amount: ZeroCoins(),
			})
		}
	}
	if len(txs) > 0 {
		incomes = append(incomes, ExternalIncome{
			Utime:  txs[0].Now, // mark unknown tx with first tx time
			Lt:     txs[0].LT,
			To:     addr,
			Amount: NewCoins(amount),
		})
	}
	return incomes, nil
}

func decodeJettonTransferNotification(msg *tlb.InternalMessage) (jettonTransferNotificationMsg, error) {
	if msg == nil {
		return jettonTransferNotificationMsg{}, fmt.Errorf("nil msg")
	}
	payload := msg.Payload()
	if payload == nil {
		return jettonTransferNotificationMsg{}, fmt.Errorf("empty payload")
	}
	var notification struct {
		_              tlb.Magic        `tlb:"#7362d09c"`
		QueryID        uint64           `tlb:"## 64"`
		Amount         tlb.Coins        `tlb:"."`
		Sender         *address.Address `tlb:"addr"`
		ForwardPayload *cell.Cell       `tlb:"either . ^"`
	}
	err := tlb.LoadFromCell(&notification, payload.BeginParse())
	if err != nil {
		return jettonTransferNotificationMsg{}, err
	}
	return jettonTransferNotificationMsg{
		Sender:  notification.Sender,
		Amount:  NewCoins(notification.Amount.NanoTON()),
		Comment: LoadComment(notification.ForwardPayload),
	}, nil
}

func DecodeJettonTransfer(msg *tlb.InternalMessage) (JettonTransferMsg, error) {
	if msg == nil {
		return JettonTransferMsg{}, fmt.Errorf("nil msg")
	}
	payload := msg.Payload()
	if payload == nil {
		return JettonTransferMsg{}, fmt.Errorf("empty payload")
	}
	var transfer struct {
		_                   tlb.Magic        `tlb:"#0f8a7ea5"`
		QueryID             uint64           `tlb:"## 64"`
		Amount              tlb.Coins        `tlb:"."`
		Destination         *address.Address `tlb:"addr"`
		ResponseDestination *address.Address `tlb:"addr"`
		CustomPayload       *cell.Cell       `tlb:"maybe ^"`
		ForwardTonAmount    tlb.Coins        `tlb:"."`
		ForwardPayload      *cell.Cell       `tlb:"either . ^"`
	}
	err := tlb.LoadFromCell(&transfer, payload.BeginParse())
	if err != nil {
		return JettonTransferMsg{}, err
	}
	return JettonTransferMsg{
		NewCoins(transfer.Amount.NanoTON()),
		transfer.Destination,
		LoadComment(transfer.ForwardPayload),
	}, nil
}

func decodeJettonExcesses(msg *tlb.InternalMessage) (uint64, error) {
	if msg == nil {
		return 0, fmt.Errorf("nil msg")
	}
	payload := msg.Payload()
	if payload == nil {
		return 0, fmt.Errorf("empty payload")
	}
	var excesses struct {
		_       tlb.Magic `tlb:"#d53276db"`
		QueryID uint64    `tlb:"## 64"`
	}
	err := tlb.LoadFromCell(&excesses, payload.BeginParse())
	if err != nil {
		return 0, err
	}
	return excesses.QueryID, nil
}

func parseExternalMessage(msg *tlb.ExternalMessage) (
	u uuid.UUID,
	addrMap map[Address]struct{},
	isValidWithdrawal bool,
	err error,
) {
	if msg == nil {
		return uuid.UUID{}, nil, false, fmt.Errorf("nil msg")
	}
	addrMap = make(map[Address]struct{})

	info, err := getHighLoadWalletExtMsgInfo(msg)
	if err != nil {
		return uuid.UUID{}, nil, false, err
	}

	for _, m := range info.Messages.All() {
		var (
			intMsg tlb.InternalMessage
			addr   Address
		)
		msgCell, err := m.Value.BeginParse().LoadRef()
		if err != nil {
			return uuid.UUID{}, nil, false, err
		}
		err = tlb.LoadFromCell(&intMsg, msgCell)
		if err != nil {
			return uuid.UUID{}, nil, false, err
		}
		jettonTransfer, err := DecodeJettonTransfer(&intMsg)
		if err == nil {
			addr, err = AddressFromTonutilsAddress(jettonTransfer.Destination)
			if err != nil {
				return uuid.UUID{}, nil, false, nil
			}
		} else {
			addr, err = AddressFromTonutilsAddress(intMsg.DstAddr)
			if err != nil {
				return uuid.UUID{}, nil, false, nil
			}
		}
		_, ok := addrMap[addr]
		if ok { // not unique addresses
			return uuid.UUID{}, nil, false, nil
		}
		addrMap[addr] = struct{}{}
	}
	return info.UUID, addrMap, true, nil
}

func (s *BlockScanner) failedWithdrawals(inMap map[Address]struct{}, outMap map[Address]struct{}, u uuid.UUID) []ExternalWithdrawal {
	var w []ExternalWithdrawal
	for i := range inMap {
		_, dstOk := s.db.GetWalletType(i)
		if _, ok := outMap[i]; !ok && !dstOk { // !dstOk - not failed internal fee payments
			w = append(w, ExternalWithdrawal{ExtMsgUuid: u, To: i, IsFailed: true})
		}
	}
	return w
}

func getHighLoadWalletExtMsgInfo(extMsg *tlb.ExternalMessage) (HighLoadWalletExtMsgInfo, error) {
	body := extMsg.Payload()
	if body == nil {
		return HighLoadWalletExtMsgInfo{}, fmt.Errorf("nil body for external message")
	}
	hash := body.Hash() // must be 32 bytes
	u, err := uuid.FromBytes(hash[:16])
	if err != nil {
		return HighLoadWalletExtMsgInfo{}, err
	}

	var data struct {
		Sign        []byte           `tlb:"bits 512"`
		SubwalletID uint32           `tlb:"## 32"`
		BoundedID   uint64           `tlb:"## 64"`
		Messages    *cell.Dictionary `tlb:"dict 16"`
	}
	err = tlb.LoadFromCell(&data, body.BeginParse())
	if err != nil {
		return HighLoadWalletExtMsgInfo{}, err
	}
	ttl := time.Unix(int64((data.BoundedID>>32)&0x00_00_00_00_FF_FF_FF_FF), 0)
	return HighLoadWalletExtMsgInfo{UUID: u, TTL: ttl, Messages: data.Messages}, nil
}

func (s *BlockScanner) processTonHotWalletExternalInMsg(tx *tlb.Transaction) (Events, error) {
	var events Events
	inMsg := tx.IO.In.AsExternalIn()
	// withdrawal messages must be only with different recipients for identification
	u, addrMapIn, isValid, err := parseExternalMessage(inMsg)
	if err != nil {
		return Events{}, err
	}
	if !isValid {
		audit.LogTX(audit.Error, string(TonHotWallet), tx.Hash, "not valid external message")
		return Events{}, fmt.Errorf("not valid message")
	}

	addrMapOut := make(map[Address]struct{})
	for _, m := range tx.IO.Out {
		if m.MsgType != tlb.MsgTypeInternal {
			audit.LogTX(audit.Error, string(TonHotWallet), tx.Hash, "not internal out message for transaction")
			return Events{}, fmt.Errorf("anomalous behavior of the TON hot wallet")
		}
		msg := m.AsInternal()

		addr, err := AddressFromTonutilsAddress(msg.DstAddr)
		if err != nil {
			return Events{}, fmt.Errorf("invalid address in withdrawal message")
		}
		dstType, dstOk := s.db.GetWalletTypeByTonutilsAddress(msg.DstAddr)

		if dstOk && dstType == JettonHotWallet { // Jetton external withdrawal
			jettonTransfer, err := DecodeJettonTransfer(msg)
			if err != nil {
				audit.LogTX(audit.Error, string(TonHotWallet), tx.Hash, "invalid jetton transfer message to hot jetton wallet")
				return Events{}, fmt.Errorf("invalid jetton transfer message to hot jetton wallet")
			}
			a, err := AddressFromTonutilsAddress(jettonTransfer.Destination)
			if err != nil {
				return Events{}, fmt.Errorf("invalid address in withdrawal message")
			}
			events.ExternalWithdrawals = append(events.ExternalWithdrawals, ExternalWithdrawal{
				ExtMsgUuid: u,
				Utime:      msg.CreatedAt,
				Lt:         msg.CreatedLT,
				To:         a,
				Amount:     jettonTransfer.Amount,
				Comment:    jettonTransfer.Comment,
				IsFailed:   false,
			})
			addrMapOut[a] = struct{}{}
			continue
		}

		if dstOk && dstType == JettonOwner { // Jetton internal withdrawal or service withdrawal
			e, err := s.processTonHotWalletProxyMsg(msg)
			if err != nil {
				return Events{}, fmt.Errorf("jetton withdrawal error: %v", err)
			}
			events.Append(e)
			addrMapOut[addr] = struct{}{}
			continue
		}

		if !dstOk { // hot_wallet -> unknown_address. to filter internal fee payments
			events.ExternalWithdrawals = append(events.ExternalWithdrawals, ExternalWithdrawal{
				ExtMsgUuid: u,
				Utime:      msg.CreatedAt,
				Lt:         msg.CreatedLT,
				To:         addr,
				Amount:     NewCoins(msg.Amount.NanoTON()),
				Comment:    msg.Comment(),
				IsFailed:   false,
			})
		}
		addrMapOut[addr] = struct{}{}
	}
	events.ExternalWithdrawals = append(events.ExternalWithdrawals, s.failedWithdrawals(addrMapIn, addrMapOut, u)...)
	return events, nil
}

func (s *BlockScanner) processTonHotWalletProxyMsg(msg *tlb.InternalMessage) (Events, error) {
	var events Events
	body := msg.Payload()
	internalPayload, err := body.BeginParse().LoadRef()
	if err != nil {
		return Events{}, fmt.Errorf("no internal payload to proxy contract: %v", err)
	}
	var intMsg tlb.InternalMessage
	err = tlb.LoadFromCell(&intMsg, internalPayload)
	if err != nil {
		return Events{}, fmt.Errorf("can not decode payload message for proxy contract: %v", err)
	}

	destType, ok := s.db.GetWalletTypeByTonutilsAddress(intMsg.DstAddr)
	// ok && destType == TonHotWallet - service TON withdrawal
	// !ok - service Jetton withdrawal
	if ok && destType == JettonDepositWallet { // Jetton internal withdrawal
		jettonTransfer, err := DecodeJettonTransfer(&intMsg)
		if err != nil {
			return Events{}, fmt.Errorf("invalid jetton transfer message to deposit jetton wallet: %v", err)
		}
		a, err := AddressFromTonutilsAddress(jettonTransfer.Destination)
		if err != nil {
			return Events{}, fmt.Errorf("invalid address in withdrawal message")
		}
		events.SendingConfirmations = append(events.SendingConfirmations, SendingConfirmation{
			Lt:   msg.CreatedLT,
			From: a,
			Memo: jettonTransfer.Comment,
		})
	}
	return events, nil
}

func (s *BlockScanner) processTonHotWalletInternalInMsg(tx *tlb.Transaction) (Events, error) {
	var events Events
	inMsg := tx.IO.In.AsInternal()
	srcAddr, err := AddressFromTonutilsAddress(inMsg.SrcAddr)
	if err != nil {
		return Events{}, err
	}
	dstAddr, err := AddressFromTonutilsAddress(inMsg.DstAddr)
	if err != nil {
		return Events{}, err
	}

	srcType, srcOk := s.db.GetWalletType(srcAddr)
	if !srcOk { // unknown_address -> hot_wallet. to check for external jetton transfer confirmation via excess message
		queryID, err := decodeJettonExcesses(inMsg)
		if err == nil {
			events.WithdrawalConfirmations = append(events.WithdrawalConfirmations,
				JettonWithdrawalConfirmation{queryID})
		}
	} else if srcOk && srcType == TonDepositWallet { // income TONs from deposit
		income := InternalIncome{
			Lt:       inMsg.CreatedLT,
			Utime:    inMsg.CreatedAt,
			From:     srcAddr,
			To:       dstAddr,
			Amount:   NewCoins(inMsg.Amount.NanoTON()),
			Memo:     inMsg.Comment(),
			IsFailed: false,
		}
		success, err := checkTxForSuccess(tx)
		if err != nil {
			return Events{}, err
		}
		// TODO: check for partially failed message
		if success {
			events.InternalIncomes = append(events.InternalIncomes, income)
		} else {
			income.IsFailed = true
			events.InternalIncomes = append(events.InternalIncomes, income)
		}
	} else if srcOk && srcType == JettonHotWallet { // income Jettons notification from Jetton hot wallet
		income, err := decodeJettonTransferNotification(inMsg)
		if err == nil {
			sender, err := AddressFromTonutilsAddress(income.Sender)
			if err != nil {
				return Events{}, err
			}
			fromType, fromOk := s.db.GetWalletType(sender)
			if !fromOk || fromType != JettonOwner { // skip transfers not from deposit wallets
				return events, nil
			}
			events.InternalIncomes = append(events.InternalIncomes, InternalIncome{
				Lt:       inMsg.CreatedLT,
				Utime:    inMsg.CreatedAt,
				From:     sender, // sender == owner of jetton deposit wallet
				To:       srcAddr,
				Amount:   income.Amount,
				Memo:     income.Comment,
				IsFailed: false,
			})
		}
	}
	return events, nil
}

func (s *BlockScanner) processTonDepositWalletExternalInMsg(tx *tlb.Transaction) (Events, error) {
	var events Events

	dstAddr, err := AddressFromTonutilsAddress(tx.IO.In.AsExternalIn().DstAddr)
	if err != nil {
		return Events{}, err
	}

	for _, o := range tx.IO.Out {
		if o.MsgType != tlb.MsgTypeInternal {
			audit.LogTX(audit.Error, string(TonDepositWallet), tx.Hash, "not internal out message for transaction")
			return Events{}, fmt.Errorf("anomalous behavior of the deposit TON wallet")
		}
		msg := o.AsInternal()
		t, srcOk := s.db.GetWalletTypeByTonutilsAddress(msg.DstAddr)
		if !srcOk || t != TonHotWallet {
			audit.LogTX(audit.Warning, string(TonDepositWallet), tx.Hash, fmt.Sprintf("TONs withdrawal from %v to %v (not to hot wallet)",
				msg.SrcAddr.String(), msg.DstAddr.String()))
			continue
		}
		events.SendingConfirmations = append(events.SendingConfirmations, SendingConfirmation{
			Lt:   msg.CreatedLT,
			From: dstAddr,
			Memo: msg.Comment(),
		})
		events.InternalWithdrawals = append(events.InternalWithdrawals, InternalWithdrawal{
			Utime:    msg.CreatedAt,
			Lt:       msg.CreatedLT,
			From:     dstAddr,
			Amount:   NewCoins(msg.Amount.NanoTON()),
			Memo:     msg.Comment(),
			IsFailed: false,
		})
	}
	return events, nil
}

func (s *BlockScanner) processTonDepositWalletInternalInMsg(tx *tlb.Transaction) (Events, error) {
	var (
		events Events
		from   Address
		err    error
	)

	inMsg := tx.IO.In.AsInternal()
	dstAddr, err := AddressFromTonutilsAddress(inMsg.DstAddr)
	if err != nil {
		return Events{}, err
	}

	isKnownSender := false
	if inMsg.SrcAddr.Type() == address.StdAddress {
		from, err = AddressFromTonutilsAddress(inMsg.SrcAddr)
		if err != nil {
			return Events{}, err
		}
		_, isKnownSender = s.db.GetWalletType(from)
	}
	if !isKnownSender { // income TONs from payer. exclude internal (hot->deposit, deposit->deposit) transfers.
		events.ExternalIncomes = append(events.ExternalIncomes, ExternalIncome{
			Lt:      inMsg.CreatedLT,
			Utime:   inMsg.CreatedAt,
			From:    from.ToBytes(),
			To:      dstAddr,
			Amount:  NewCoins(inMsg.Amount.NanoTON()),
			Comment: inMsg.Comment(),
		})
	}
	return events, nil
}

func (s *BlockScanner) processJettonDepositOutMsgs(tx *tlb.Transaction) (Events, *big.Int, bool, error) {
	var events Events
	knownIncomeAmount := big.NewInt(0)
	unknownMsgFound := false

	for _, m := range tx.IO.Out { // checks for JettonTransferNotification

		if m.MsgType != tlb.MsgTypeInternal {
			audit.LogTX(audit.Info, string(JettonDepositWallet), tx.Hash, "sends external out message")
			unknownMsgFound = true
			continue
		} // skip external_out msg

		outMsg := m.AsInternal()
		srcAddr, err := AddressFromTonutilsAddress(outMsg.SrcAddr)
		if err != nil {
			return Events{}, nil, false, err
		}

		notify, err := decodeJettonTransferNotification(outMsg)
		if err != nil {
			unknownMsgFound = true
			continue
		}

		// need not check success. impossible for failed txs.
		_, senderOk := s.db.GetWalletTypeByTonutilsAddress(notify.Sender)
		if senderOk {
			// TODO: check balance calculation for unknown transactions for service transfers
			audit.LogTX(audit.Info, string(JettonDepositWallet), tx.Hash, "service Jetton transfer")
			// not set unknownMsgFound = true to prevent service transfers interpretation as unknown
			continue
		} // some kind of internal transfer

		dstAddr, err := AddressFromTonutilsAddress(outMsg.DstAddr)
		if err != nil {
			return Events{}, nil, false, err
		}
		owner := s.db.GetOwner(srcAddr)
		if owner == nil {
			return Events{}, nil, false, fmt.Errorf("no owner for Jetton deposit in addressbook")
		}
		if dstAddr != *owner {
			audit.LogTX(audit.Info, string(JettonDepositWallet), tx.Hash,
				"sends transfer notification message not to owner")
			// interpret it as an unknown message
			unknownMsgFound = true
			continue
		}

		var from []byte
		if notify.Sender != nil &&
			(notify.Sender.Type() == address.StdAddress || notify.Sender.Type() == address.VarAddress) {
			from = notify.Sender.Data()
		}
		events.ExternalIncomes = append(events.ExternalIncomes, ExternalIncome{
			Utime:   outMsg.CreatedAt,
			Lt:      outMsg.CreatedLT,
			From:    from,
			To:      srcAddr,
			Amount:  notify.Amount,
			Comment: notify.Comment,
		})
		knownIncomeAmount.Add(knownIncomeAmount, notify.Amount.BigInt())
	}
	return events, knownIncomeAmount, unknownMsgFound, nil
}

func (s *BlockScanner) processJettonDepositInMsg(tx *tlb.Transaction) (Events, *big.Int, bool, error) {
	var events Events
	unknownMsgFound := false
	totalWithdrawalsAmount := big.NewInt(0)

	if tx.IO.In == nil { // skip not decodable in_msg
		audit.LogTX(audit.Info, string(JettonDepositWallet), tx.Hash, "transaction without in message")
		// interpret it as an unknown message
		return events, totalWithdrawalsAmount, true, nil
	}

	if tx.IO.In.MsgType != tlb.MsgTypeInternal { // skip not decodable in_msg
		audit.LogTX(audit.Info, string(JettonDepositWallet), tx.Hash, "not internal in message")
		// interpret it as an unknown message
		return events, totalWithdrawalsAmount, true, nil
	}

	success, err := checkTxForSuccess(tx)
	if err != nil {
		return Events{}, nil, false, err
	}

	inMsg := tx.IO.In.AsInternal()
	dstAddr, err := AddressFromTonutilsAddress(inMsg.DstAddr)
	if err != nil {
		return Events{}, nil, false, err
	}

	transfer, err := DecodeJettonTransfer(inMsg)
	if err != nil {
		unknownMsgFound = true
		return events, totalWithdrawalsAmount, unknownMsgFound, nil
	}

	if !success { // failed withdrawal from deposit jetton wallet
		events.InternalWithdrawals = append(events.InternalWithdrawals, InternalWithdrawal{
			Utime:    inMsg.CreatedAt,
			Lt:       inMsg.CreatedLT,
			From:     dstAddr,
			Amount:   transfer.Amount,
			Memo:     transfer.Comment,
			IsFailed: true,
		})
		return events, totalWithdrawalsAmount, unknownMsgFound, nil
	}

	// success withdrawal from deposit jetton wallet
	if len(tx.IO.Out) < 1 {
		audit.LogTX(audit.Error, string(JettonDepositWallet), tx.Hash, "success Jettons transfer TX without out message")
		return Events{}, nil, true, fmt.Errorf("anomalous behavior of the deposit Jetton wallet")
	}
	totalWithdrawalsAmount.Add(totalWithdrawalsAmount, transfer.Amount.BigInt())
	destType, destOk := s.db.GetWalletTypeByTonutilsAddress(transfer.Destination)
	if !destOk || destType != TonHotWallet {
		audit.LogTX(audit.Warning, string(JettonDepositWallet), tx.Hash,
			fmt.Sprintf("Jettons withdrawal from %v to %v (not to hot wallet)",
				inMsg.DstAddr.String(), transfer.Destination.String()))
		// TODO: check balance calculation for unknown transactions for service transfers
		// not set unknownMsgFound = true to prevent service transfers interpretation as unknown
		return Events{}, totalWithdrawalsAmount, false, nil
	}
	events.InternalWithdrawals = append(events.InternalWithdrawals, InternalWithdrawal{
		Utime:    inMsg.CreatedAt,
		Lt:       inMsg.CreatedLT,
		From:     dstAddr,
		Amount:   transfer.Amount,
		Memo:     transfer.Comment,
		IsFailed: false,
	})

	return events, totalWithdrawalsAmount, unknownMsgFound, nil
}
