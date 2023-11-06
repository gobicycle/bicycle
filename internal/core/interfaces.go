package core

import (
	"context"
	"github.com/google/uuid"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
	"github.com/xssnick/tonutils-go/address"
	"math/big"
	"time"
)

type storage interface {
	GetExternalWithdrawalTasks(ctx context.Context, limit int) ([]ExternalWithdrawalTask, error)
	SaveTonWallet(ctx context.Context, walletData WalletData) error
	SaveJettonWallet(ctx context.Context, ownerAddress Address, walletData WalletData, notSaveOwner bool) error
	GetWalletType(address Address) (WalletType, bool)
	GetOwner(address Address) *Address
	GetWalletTypeByTonutilsAddress(address *address.Address) (WalletType, bool)
	SaveParsedBlocksData(ctx context.Context, events []BlockEvents, shardBlocks []*ShardBlock, masterBlock tlb.Block) error
	GetTonInternalWithdrawalTasks(ctx context.Context, limit int) ([]InternalWithdrawalTask, error)
	GetJettonInternalWithdrawalTasks(ctx context.Context, forbiddenAddresses []Address, limit int) ([]InternalWithdrawalTask, error)
	CreateExternalWithdrawals(ctx context.Context, tasks []ExternalWithdrawalTask, extMsgUuid uuid.UUID, expiredAt time.Time) error
	GetTonHotWalletAddress(ctx context.Context) (Address, error)
	SetExpired(ctx context.Context) error
	SaveInternalWithdrawalTask(ctx context.Context, task InternalWithdrawalTask, expiredAt time.Time, memo uuid.UUID) error
	IsActualBlockData(ctx context.Context) (bool, error)
	SaveWithdrawalRequest(ctx context.Context, w WithdrawalRequest) (int64, error)
	IsInProgressInternalWithdrawalRequest(ctx context.Context, dest Address, currency string) (bool, error)
	GetServiceHotWithdrawalTasks(ctx context.Context, limit int) ([]ServiceWithdrawalTask, error)
	UpdateServiceWithdrawalRequest(ctx context.Context, t ServiceWithdrawalTask, tonAmount Coins,
		expiredAt time.Time, filled bool) error
	GetServiceDepositWithdrawalTasks(ctx context.Context, limit int) ([]ServiceWithdrawalTask, error)
	GetJettonWallet(ctx context.Context, address Address) (*WalletData, bool, error)
}

type blockchain interface {
	GetJettonWalletAddress(ctx context.Context, owner tongo.AccountID, jettonMaster tongo.AccountID) (*tongo.AccountID, error)
	//GetTransactionIDsFromBlock(ctx context.Context, blockID *ton.BlockIDExt) ([]ton.TransactionShortInfo, error)
	//GetTransactionFromBlock(ctx context.Context, blockID *ton.BlockIDExt, txID ton.TransactionShortInfo) (*tlb.Transaction, error)
	GenerateDefaultWallet(seed string, isHighload bool) (*wallet.Wallet, uint32, error)
	GetJettonBalance(ctx context.Context, jettonWallet tongo.AccountID, blockID tongo.BlockIDExt) (*big.Int, error)
	//SendExternalMessage(ctx context.Context, msg *tlb.ExternalMessage) error // TODO: fix
	GetAccountCurrentState(ctx context.Context, address tongo.AccountID) (uint64, tlb.AccountStatus, error)
	GetLastJettonBalance(ctx context.Context, jettonWallet tongo.AccountID) (*big.Int, error)
	DeployTonWallet(ctx context.Context, wallet *wallet.Wallet) error
}

type blocksTracker interface {
	NextBatch() ([]*ShardBlock, *tlb.Block, error)
}

type Notificator interface {
	Publish(payload any) error
}
