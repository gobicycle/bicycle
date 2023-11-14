package api

import (
	"context"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/google/uuid"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
)

type storage interface {
	SaveTonWallet(ctx context.Context, walletData core.WalletData) error
	SaveJettonWallet(ctx context.Context, ownerAddress core.Address, walletData core.WalletData, notSaveOwner bool) error
	GetTonWalletsAddresses(ctx context.Context, userID string, types []core.WalletType) ([]core.Address, error)
	GetJettonOwnersAddresses(ctx context.Context, userID string, types []core.WalletType) ([]core.OwnerWallet, error)
	SaveWithdrawalRequest(ctx context.Context, w core.WithdrawalRequest) (int64, error)
	IsWithdrawalRequestUnique(ctx context.Context, w core.WithdrawalRequest) (bool, error)
	IsActualBlockData(ctx context.Context) (bool, error)
	GetExternalWithdrawalStatus(ctx context.Context, id int64) (core.WithdrawalStatus, error)
	GetWalletType(address core.Address) (core.WalletType, bool)
	GetIncome(ctx context.Context, userID string, isDepositSide bool) ([]core.TotalIncome, error)
	SaveServiceWithdrawalRequest(ctx context.Context, w core.ServiceWithdrawalRequest) (uuid.UUID, error)
	GetIncomeHistory(ctx context.Context, userID string, currency string, limit int, offset int) ([]core.ExternalIncome, error)
	GetOwner(address core.Address) *core.Address
}

type blockchain interface {
	RunSmcMethod(ctx context.Context, accountID ton.AccountID, method string, params tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethodByID(ctx context.Context, accountID ton.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error)
}
