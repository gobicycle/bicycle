package api

import (
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/oas"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/xssnick/tonutils-go/address"
)

func convertWithdrawal(w *oas.SendWithdrawalReq) (*core.WithdrawalRequest, error) {
	if !isValidCurrency(w.Currency) {
		return nil, fmt.Errorf("invalid currency")
	}
	addr, bounceable, err := validateAddress(w.Destination)
	if err != nil {
		return nil, fmt.Errorf("invalid destination address: %w", err)
	}
	if !(w.Amount.Cmp(decimal.New(0, 0)) == 1) {
		return nil, fmt.Errorf("amount must be > 0")
	}
	return &core.WithdrawalRequest{
		UserID:      w.UserID,
		QueryID:     w.QueryID,
		Currency:    w.Currency,
		Amount:      w.Amount,
		Destination: addr,
		Bounceable:  bounceable,
		Comment:     w.Comment,
		IsInternal:  false,
	}, nil
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

func convertTonServiceWithdrawal(s storage, w *oas.ServiceTonWithdrawalReq) (*core.ServiceWithdrawalRequest, error) {
	from, _, err := validateAddress(w.From)
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

func convertJettonServiceWithdrawal(s storage, w *oas.ServiceJettonWithdrawalReq) (*core.ServiceWithdrawalRequest, error) {
	from, _, err := validateAddress(w.Owner)
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
	jetton, _, err := validateAddress(w.JettonMaster)
	if err != nil {
		return nil, fmt.Errorf("invalid jetton master address: %v", err)
	}
	// currency type checks by withdrawal processor
	return &core.ServiceWithdrawalRequest{
		From:         from,
		JettonMaster: &jetton,
	}, nil
}

func convertIncome(dbConn storage, totalIncomes []core.TotalIncome) *oas.CalculatedIncome {
	var res = GetIncomeResponse{
		TotalIncomes: []totalIncome{},
	}
	if config.Config.IsDepositSideCalculation {
		res.Side = core.SideDeposit
	} else {
		res.Side = core.SideHotWallet
	}

	for _, b := range totalIncomes {
		totIncome := totalIncome{
			Amount:   b.Amount.String(),
			Currency: b.Currency,
		}
		if b.Currency == core.TonSymbol {
			totIncome.Address = b.Deposit.ToUserFormat()
		} else {
			owner := dbConn.GetOwner(b.Deposit)
			if owner == nil {
				// TODO: remove fatal
				log.Fatalf("can not find owner for deposit: %s", b.Deposit.ToUserFormat())
			}
			totIncome.Address = owner.ToUserFormat()
		}
		res.TotalIncomes = append(res.TotalIncomes, totIncome)
	}
	return res
}

func convertHistory(dbConn storage, currency string, incomes []core.ExternalIncome) *oas.Incomes {
	var res = GetHistoryResponse{
		Incomes: []income{},
	}
	for _, i := range incomes {
		inc := income{
			Time:    int64(i.Utime),
			Amount:  i.Amount.String(),
			Comment: i.Comment,
		}
		if currency == core.TonSymbol {
			inc.DepositAddress = i.To.ToUserFormat()
		} else {
			owner := dbConn.GetOwner(i.To)
			if owner == nil {
				// TODO: remove fatal
				log.Fatalf("can not find owner for deposit: %s", i.To.ToUserFormat())
			}
			inc.DepositAddress = owner.ToUserFormat()
		}
		// show only std address
		if len(i.From) == 32 && i.FromWorkchain != nil {
			addr := address.NewAddress(0, byte(*i.FromWorkchain), i.From)
			addr.SetTestnetOnly(config.Config.Testnet)
			inc.SourceAddress = addr.String()
		}
		res.Incomes = append(res.Incomes, inc)
	}
	return res
}

func generateDeposit(
	ctx context.Context,
	userID string,
	currency string,
	shard tongo.ShardID,
	dbConn storage,
	bc blockchain,
	hotWalletAddress address.Address,
) (
	*oas.Deposit,
	error,
) {
	subwalletID, err := dbConn.GetLastSubwalletID(ctx)
	if err != nil {
		return nil, err
	}
	var res string
	if currency == core.TonSymbol {
		w, id, err := bc.GenerateSubWallet(config.Config.Seed, shard, subwalletID+1)
		if err != nil {
			return nil, err
		}
		a, err := core.AddressFromTonutilsAddress(w.GetAddress())
		if err != nil {
			return nil, err
		}
		err = dbConn.SaveTonWallet(ctx,
			core.WalletData{
				SubwalletID: id,
				UserID:      userID,
				Currency:    core.TonSymbol,
				Type:        core.TonDepositWallet,
				Address:     a,
			},
		)
		if err != nil {
			return "", err
		}
		res = a.ToUserFormat()
	} else {
		jetton, ok := config.Config.Jettons[currency]
		if !ok {
			return "", fmt.Errorf("jetton address not found")
		}
		proxy, addr, err := bc.GenerateDepositJettonWalletForProxy(ctx, shard, &hotWalletAddress, jetton.Master, subwalletID+1)
		if err != nil {
			return "", err
		}
		jettonWalletAddr, err := core.AddressFromTonutilsAddress(addr)
		if err != nil {
			return "", err
		}
		proxyAddr, err := core.AddressFromTonutilsAddress(proxy.Address())
		if err != nil {
			return "", err
		}
		err = dbConn.SaveJettonWallet(
			ctx,
			proxyAddr,
			core.WalletData{
				UserID:      userID,
				SubwalletID: proxy.SubwalletID,
				Currency:    currency,
				Type:        core.JettonDepositWallet,
				Address:     jettonWalletAddr,
			},
			false,
		)
		if err != nil {
			return "", err
		}
		res = proxyAddr.ToUserFormat()
	}
	return res, nil
}

func getDeposits(ctx context.Context, userID string, dbConn storage) (*oas.Deposits, error) {
	var res = GetAddressesResponse{
		Addresses: []WalletAddress{},
	}
	tonAddr, err := dbConn.GetTonWalletsAddresses(ctx, userID, []core.WalletType{core.TonDepositWallet})
	if err != nil {
		return GetAddressesResponse{}, err
	}
	jettonAddr, err := dbConn.GetJettonOwnersAddresses(ctx, userID, []core.WalletType{core.JettonDepositWallet})
	if err != nil {
		return GetAddressesResponse{}, err
	}
	for _, a := range tonAddr {
		res.Addresses = append(res.Addresses, WalletAddress{Address: a.ToUserFormat(), Currency: core.TonSymbol})
	}
	for _, a := range jettonAddr {
		res.Addresses = append(res.Addresses, WalletAddress{Address: a.Address.ToUserFormat(), Currency: a.Currency})
	}
	return res, nil
}

func isValidCurrency(cur string) bool {
	if _, ok := config.Config.Jettons[cur]; ok || cur == core.TonSymbol {
		return true
	}
	return false
}

func validateAddress(addr string) (core.Address, bool, error) {
	if addr == "" {
		return core.Address{}, false, fmt.Errorf("empty address")
	}
	a, err := address.ParseAddr(addr)
	if err != nil {
		return core.Address{}, false, fmt.Errorf("invalid address: %v", err)
	}
	if a.IsTestnetOnly() && !config.Config.Testnet {
		return core.Address{}, false, fmt.Errorf("address for testnet only")
	}
	if a.Workchain() != core.DefaultWorkchain {
		return core.Address{}, false, fmt.Errorf("address must be in %d workchain",
			core.DefaultWorkchain)
	}
	res, err := core.AddressFromTonutilsAddress(a)
	return res, a.IsBounceable(), err
}
