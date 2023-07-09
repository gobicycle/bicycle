package api

import (
	"context"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/xssnick/tonutils-go/address"
)

type storage interface {
	GetLastSubwalletID(ctx context.Context) (uint32, error)
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

type blockchainExecutor interface {
	GenerateSubWallet(seed string, shard tongo.ShardID, startSubWalletID uint32) (*wallet.Wallet, uint32, error)
	GenerateDepositJettonWalletForProxy(
		ctx context.Context,
		shard tongo.ShardID,
		proxyOwner, jettonMaster *address.Address,
		startSubWalletID uint32,
	) (
		proxy *core.JettonProxy,
		addr *address.Address,
		err error,
	)
	GenerateDefaultWallet(seed string, isHighload bool) (*wallet.Wallet, uint32, error)
	RunSmcMethod(context.Context, tongo.AccountID, string, tlb.VmStack) (uint32, tlb.VmStack, error)
	RunSmcMethodByID(context.Context, tongo.AccountID, int, tlb.VmStack) (uint32, tlb.VmStack, error)
}
