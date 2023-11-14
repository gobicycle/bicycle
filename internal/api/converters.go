package api

import (
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/oas"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
)

func convertWithdrawal(w *oas.SendWithdrawalReq, isTestnet bool) (*core.WithdrawalRequest, error) {

	if !isValidCurrency(w.Currency) {
		return nil, fmt.Errorf("invalid currency")
	}

	addr, bounceable, err := validateAddress(w.Destination, isTestnet)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}

	amount, err := decimal.NewFromString(w.Amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount string: %w", err)
	}

	if !(amount.Cmp(decimal.New(0, 0)) == 1) {
		return nil, fmt.Errorf("amount must be > 0")
	}

	res := core.WithdrawalRequest{
		UserID:      w.UserID,
		QueryID:     w.QueryID,
		Currency:    w.Currency,
		Amount:      amount,
		Destination: addr,
		Bounceable:  bounceable,
		IsInternal:  false,
	}

	if w.Comment.IsSet() {
		res.Comment = w.Comment.Value
	}

	return &res, nil
}

func convertWithdrawalStatus(status core.WithdrawalStatus) *oas.WithdrawalStatus {
	switch status {
	case core.PendingStatus:
		return &oas.WithdrawalStatus{Status: oas.WithdrawalStatusStatusPending}
	case core.ProcessedStatus:
		return &oas.WithdrawalStatus{Status: oas.WithdrawalStatusStatusProcessed}
	case core.ProcessingStatus:
		return &oas.WithdrawalStatus{Status: oas.WithdrawalStatusStatusProcessing}
	}
	return nil
}

func convertTonServiceWithdrawal(s storage, w *oas.ServiceTonWithdrawalReq, isTestnet bool) (*core.ServiceWithdrawalRequest, error) {
	from, _, err := validateAddress(w.From, isTestnet)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %w", err)
	}
	t, ok := s.GetWalletType(from)
	if !ok {
		return nil, fmt.Errorf("unknown deposit address")
	}
	if t != core.JettonOwner {
		return nil,
			fmt.Errorf("service withdrawal allowed only for Jetton deposit owner")
	}
	return &core.ServiceWithdrawalRequest{
		From: from,
	}, nil
}

func convertJettonServiceWithdrawal(s storage, w *oas.ServiceJettonWithdrawalReq, isTestnet bool) (*core.ServiceWithdrawalRequest, error) {
	from, _, err := validateAddress(w.Owner, isTestnet)
	if err != nil {
		return nil, fmt.Errorf("invalid from address: %v", err)
	}
	t, ok := s.GetWalletType(from)
	if !ok {
		return nil, fmt.Errorf("unknown deposit address")
	}
	if t != core.JettonOwner && t != core.TonDepositWallet {
		return nil,
			fmt.Errorf("service withdrawal allowed only for Jetton deposit owner or TON deposit")
	}
	jetton, _, err := validateAddress(w.JettonMaster, isTestnet)
	if err != nil {
		return nil, fmt.Errorf("invalid jetton master address: %v", err)
	}
	// currency type checks by withdrawal processor
	return &core.ServiceWithdrawalRequest{
		From:         from,
		JettonMaster: &jetton,
	}, nil
}

func convertIncome(dbConn storage, totalIncomes []core.TotalIncome, isDepositSideCalculation bool) *oas.CalculatedIncome {
	var res = oas.CalculatedIncome{
		TotalIncome: []oas.TotalIncome{},
	}

	if isDepositSideCalculation {
		res.CountingSide = oas.CalculatedIncomeCountingSideDeposit
	} else {
		res.CountingSide = oas.CalculatedIncomeCountingSideHotWallet
	}

	for _, b := range totalIncomes {
		totIncome := oas.TotalIncome{
			Amount:   b.Amount.String(),
			Currency: b.Currency,
		}
		if b.Currency == core.TonSymbol {
			totIncome.DepositAddress = b.Deposit.ToUserFormat()
		} else {
			owner := dbConn.GetOwner(b.Deposit)
			if owner == nil {
				// TODO: remove panic
				panic("can not find owner for deposit: " + b.Deposit.ToUserFormat())
			}
			totIncome.DepositAddress = owner.ToUserFormat()
		}
		res.TotalIncome = append(res.TotalIncome, totIncome)
	}
	return &res
}

func convertHistory(dbConn storage, currency string, incomes []core.ExternalIncome, isTestnet bool) *oas.Incomes {
	var res = oas.Incomes{
		Incomes: []oas.Income{},
	}
	for _, i := range incomes {
		inc := oas.Income{
			Time:   int64(i.Utime),
			Amount: i.Amount.String(),
		}
		if i.Comment != "" { // TODO: diff between empty sting and no comment
			inc.Comment.SetTo(i.Comment)
		}

		if currency == core.TonSymbol {
			inc.DepositAddress = i.To.ToUserFormat()
		} else {
			owner := dbConn.GetOwner(i.To)
			if owner == nil {
				// TODO: remove panic
				panic("can not find owner for deposit: " + i.To.ToUserFormat())
			}
			inc.DepositAddress = owner.ToUserFormat()
		}
		// show only std address
		if len(i.From) == 32 && i.FromWorkchain != nil { // TODO: maybe masterchain too
			//addr := address.NewAddress(0, byte(*i.FromWorkchain), i.From)
			var a [32]byte
			copy(a[:], i.From)
			addr := tongo.NewAccountId(*i.FromWorkchain, a)
			inc.SourceAddress = addr.ToHuman(true, isTestnet)
		}
		res.Incomes = append(res.Incomes, inc)
	}
	return &res
}

func getDeposits(ctx context.Context, userID string, dbConn storage) (*oas.Deposits, error) {
	var res = oas.Deposits{
		Deposits: []oas.Deposit{},
	}
	tonAddr, err := dbConn.GetTonWalletsAddresses(ctx, userID, []core.WalletType{core.TonDepositWallet})
	if err != nil {
		return nil, err
	}
	jettonAddr, err := dbConn.GetJettonOwnersAddresses(ctx, userID, []core.WalletType{core.JettonDepositWallet})
	if err != nil {
		return nil, err
	}
	for _, a := range tonAddr {
		res.Deposits = append(res.Deposits, oas.Deposit{Address: a.ToUserFormat(), Currency: core.TonSymbol})
	}
	for _, a := range jettonAddr {
		res.Deposits = append(res.Deposits, oas.Deposit{Address: a.Address.ToUserFormat(), Currency: a.Currency})
	}
	return &res, nil
}

func isValidCurrency(cur string) bool {
	if _, ok := config.Config.Jettons[cur]; ok || cur == core.TonSymbol {
		return true
	}
	return false
}

func validateAddress(addr string, isTestnet bool) (core.Address, bool, error) {
	if addr == "" {
		return core.Address{}, false, fmt.Errorf("empty address")
	}
	a, err := tongo.ParseAddress(addr)
	if err != nil {
		return core.Address{}, false, fmt.Errorf("invalid address: %v", err)
	}
	if a.IsTestnetOnly() && !isTestnet {
		return core.Address{}, false, fmt.Errorf("address for testnet only")
	}
	if int(a.ID.Workchain) != core.DefaultWorkchain {
		return core.Address{}, false, fmt.Errorf("address must be in %d workchain",
			core.DefaultWorkchain)
	}
	res, err := core.AddressFromTonutilsAddress(a)
	return res, a.IsBounceable(), err
}
