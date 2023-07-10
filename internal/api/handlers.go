package api

import (
	"context"
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gobicycle/bicycle/internal/config"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/oas"
)

func (h Handler) MakeNewDeposit(ctx context.Context, params oas.MakeNewDepositParams) (oas.MakeNewDepositRes, error) {
	if !isValidCurrency(params.Currency) {
		return &oas.BadRequest{Error: "invalid currency type"}, nil
	}
	h.mutex.Lock()
	defer h.mutex.Unlock() // To prevent data race
	deposit, err := generateDeposit(ctx, params.UserID, params.Currency, h.shard, h.storage, h.executor, h.hotWalletAddress)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return deposit, nil
}

func (h Handler) GetDeposits(ctx context.Context, params oas.GetDepositsParams) (oas.GetDepositsRes, error) {
	deposits, err := getDeposits(ctx, params.UserID, h.storage)
	if err != nil {
		return &oas.InternalError{Error: err.Error()}, nil
	}
	return deposits, nil
}

func (h Handler) SendWithdrawal(ctx context.Context, req *oas.SendWithdrawalReq) (oas.SendWithdrawalRes, error) {
	w, err := convertWithdrawal(req)
	if err != nil {
		return &oas.BadRequest{Error: fmt.Sprintf("convert withdrawal err: %v", err)}, nil
	}
	unique, err := h.storage.IsWithdrawalRequestUnique(ctx, w)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("check withdrawal uniquess err: %v", err)}, nil
	} else if !unique {
		return &oas.InternalError{Error: "(user_id,query_id) not unique"}, nil
	}
	_, ok := h.storage.GetWalletType(w.Destination)
	if ok {
		return &oas.InternalError{Error: "withdrawal to service internal addresses not supported"}, nil
	}
	id, err := h.storage.SaveWithdrawalRequest(ctx, w)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("save withdrawal request err: %v", err)}, nil
	}
	return &oas.WithdrawalID{ID: id}, nil
}

func (h Handler) GetSync(ctx context.Context) (oas.GetSyncRes, error) {
	isSynced, err := h.storage.IsActualBlockData(ctx)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("get sync from db err: %v", err)}, nil
	}
	return &oas.SyncStatus{IsSynced: isSynced}, nil
}

func (h Handler) GetWithdrawalStatus(ctx context.Context, params oas.GetWithdrawalStatusParams) (oas.GetWithdrawalStatusRes, error) {
	status, err := h.storage.GetExternalWithdrawalStatus(ctx, params.ID)
	if errors.Is(err, core.ErrNotFound) {
		return &oas.BadRequest{Error: "request ID not found"}, nil
	}
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("get external withdrawal status err: %v", err)}, nil
	}
	s := convertWithdrawalStatus(status)
	if s == nil {
		return &oas.InternalError{Error: "invalid withdrawal status"}, nil
	}
	return s, nil
}

func (h Handler) GetIncome(ctx context.Context, params oas.GetIncomeParams) (oas.GetIncomeRes, error) {
	totalIncomes, err := h.storage.GetIncome(ctx, params.UserID, config.Config.IsDepositSideCalculation)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("get balances err: %v", err)}, nil
	}
	return convertIncome(h.storage, totalIncomes), nil
}

func (h Handler) GetIncomeHistory(ctx context.Context, params oas.GetIncomeHistoryParams) (oas.GetIncomeHistoryRes, error) {
	if !isValidCurrency(params.Currency) {
		return &oas.BadRequest{Error: "invalid currency type"}, nil
	}
	// limit and offset must be with default values!
	history, err := h.storage.GetIncomeHistory(ctx, params.UserID, params.Currency, params.Limit.Value, params.Offset.Value)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("get history err: %v", err)}, nil
	}
	return convertHistory(h.storage, params.Currency, history), nil
}

func (h Handler) ServiceTonWithdrawal(ctx context.Context, req *oas.ServiceTonWithdrawalReq) (oas.ServiceTonWithdrawalRes, error) {
	w, err := convertTonServiceWithdrawal(h.storage, req)
	if err != nil {
		return &oas.BadRequest{Error: fmt.Sprintf("convert service withdrawal err: %v", err)}, nil
	}
	memo, err := h.storage.SaveServiceWithdrawalRequest(ctx, w)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("save service withdrawal request err: %v", err)}, nil
	}
	return &oas.ServiceWithdrawalMemo{Memo: memo}, nil
}

func (h Handler) ServiceJettonWithdrawal(ctx context.Context, req *oas.ServiceJettonWithdrawalReq) (oas.ServiceJettonWithdrawalRes, error) {
	w, err := convertJettonServiceWithdrawal(h.storage, req)
	if err != nil {
		return &oas.BadRequest{Error: fmt.Sprintf("convert service withdrawal err: %v", err)}, nil
	}
	memo, err := h.storage.SaveServiceWithdrawalRequest(ctx, w)
	if err != nil {
		return &oas.InternalError{Error: fmt.Sprintf("save service withdrawal request err: %v", err)}, nil
	}
	return &oas.ServiceWithdrawalMemo{Memo: memo}, nil
}
