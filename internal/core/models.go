package core

import (
	"database/sql/driver"
	"fmt"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/xssnick/tonutils-go/address"

	"math/big"
)

// TODO: align all structures

type IncomeSide = string

const (
	HotWalletSide IncomeSide = "hot_wallet"
	DepositSide   IncomeSide = "deposit"
)

type EventName = string

const (
	ServiceWithdrawalEvent  EventName = "service withdrawal"
	InternalWithdrawalEvent EventName = "internal withdrawal"
	ExternalWithdrawalEvent EventName = "external withdrawal"
	InitEvent               EventName = "initialization"
)

type WalletType string

const (
	TonHotWallet        WalletType = "ton_hot"
	JettonHotWallet     WalletType = "jetton_hot"
	TonDepositWallet    WalletType = "ton_deposit"
	JettonDepositWallet WalletType = "jetton_deposit"
	JettonOwner         WalletType = "owner"
)

type WithdrawalStatus string

const (
	PendingStatus    WithdrawalStatus = "pending"
	ProcessingStatus WithdrawalStatus = "processing"
	ProcessedStatus  WithdrawalStatus = "processed"
)

type Address [32]byte // supports only MsgAddressInt addr_std$10 without anycast and DefaultWorkchain workchain

// Scan implements Scanner for database/sql.
func (a *Address) Scan(src interface{}) error {
	srcB, ok := src.([]byte)
	if !ok {
		return fmt.Errorf("can't scan %T into Address", src)
	}
	if len(srcB) != 32 {
		return fmt.Errorf("can't scan []byte of len %d into Address, want %d", len(srcB), 32)
	}
	copy(a[:], srcB)
	return nil
}

// Value implements valuer for database/sql.
func (a Address) Value() (driver.Value, error) {
	return a[:], nil
}

//// ToTonutilsAddressStd implements converter to ton-utils std Address type for default workchain !
//func (a Address) ToTonutilsAddressStd(flags byte) *address.Address {
//	return address.NewAddress(flags, DefaultWorkchain, a[:])
//}

// TODO: remove
// ToUserFormat converts to user-friendly text format with testnet and bounce flags
func (a Address) ToUserFormat(isTestnet bool) string {
	addr := tongo.NewAccountId(DefaultWorkchainID, a)
	return addr.ToHuman(false, isTestnet)
}

// TongoAccountIDToUserFormat converts to user-friendly text format with testnet and bounce = false flags
func TongoAccountIDToUserFormat(addr tongo.AccountID, isTestnet bool) string { // TODO: check user format type
	return addr.ToHuman(false, isTestnet) // TODO: or use non bounceable for Jetton wallets
}

func (a Address) ToBytes() []byte {
	return a[:]
}

//func TonutilsAddressToUserFormat(addr *address.Address) string {
//	addr.SetTestnetOnly(config.Config.Testnet)
//	addr.SetBounce(false)
//	return addr.String()
//}

func AddressFromBytes(data []byte) (Address, error) {
	if len(data) != 32 {
		return Address{}, fmt.Errorf("invalid address len. Std addr len must be 32 bytes")
	}
	var res Address
	copy(res[:], data)
	return res, nil
}

//func AddressFromTonutilsAddress(addr *address.Address) (Address, error) {
//	if addr == nil {
//		return Address{}, fmt.Errorf("nil tonutils address")
//	}
//	if addr.Type() != address.StdAddress {
//		return Address{}, fmt.Errorf("only std address supported")
//	}
//	return AddressFromBytes(addr.Data())
//}

//func AddressMustFromTonutilsAddress(addr *address.Address) Address {
//	res, err := AddressFromTonutilsAddress(addr)
//	if err != nil {
//		panic(err)
//	}
//	return res
//}

type AddressInfo struct {
	Type  WalletType
	Owner *Address
}

type JettonWallet struct {
	Address  *address.Address
	Currency string
}

type OwnerWallet struct {
	Address  Address
	Currency string
}

type WalletData struct {
	SubwalletID uint32
	UserID      string
	Currency    string
	Type        WalletType
	Address     Address
}

type WithdrawalRequest struct {
	QueryID     string
	UserID      string
	Currency    string
	Amount      Coins
	Bounceable  bool
	IsInternal  bool
	Destination Address
	Comment     string
}

type ServiceWithdrawalRequest struct {
	From         Address
	JettonMaster *Address
}

type ServiceWithdrawalTask struct {
	ServiceWithdrawalRequest
	JettonAmount Coins
	Memo         uuid.UUID
	SubwalletID  uint32
}

type ExternalWithdrawalTask struct {
	QueryID     int64
	Currency    string
	Amount      Coins
	Destination Address
	Bounceable  bool
	Comment     string
}

type InternalWithdrawal struct {
	Utime    uint32
	Lt       uint64
	From     Address
	Amount   Coins
	Memo     string // uuid from comment
	IsFailed bool
}

type SendingConfirmation struct {
	Lt   uint64 // Lt of outgoing wallet message
	From Address
	Memo string // uuid from comment
}

type ExternalWithdrawal struct {
	ExtMsgUuid uuid.UUID
	Utime      uint32
	Lt         uint64
	To         Address
	Amount     Coins
	Comment    string
	IsFailed   bool
}

type JettonWithdrawalConfirmation struct {
	QueryId uint64
}

type InternalIncome struct {
	Utime    uint32
	Lt       uint64 // will not fit in db bigint after 1.5 billion years
	From     Address
	To       Address
	Amount   Coins
	Memo     string
	IsFailed bool
}

type ExternalIncome struct {
	Utime         uint32
	Lt            uint64
	From          []byte
	FromWorkchain *int32
	To            Address
	Amount        Coins
	Comment       string
}

type Events struct {
	ExternalIncomes         []ExternalIncome
	InternalIncomes         []InternalIncome
	SendingConfirmations    []SendingConfirmation
	InternalWithdrawals     []InternalWithdrawal
	ExternalWithdrawals     []ExternalWithdrawal
	WithdrawalConfirmations []JettonWithdrawalConfirmation
}

func (e *Events) Append(ae Events) {
	e.ExternalIncomes = append(e.ExternalIncomes, ae.ExternalIncomes...)
	e.InternalIncomes = append(e.InternalIncomes, ae.InternalIncomes...)
	e.SendingConfirmations = append(e.SendingConfirmations, ae.SendingConfirmations...)
	e.InternalWithdrawals = append(e.InternalWithdrawals, ae.InternalWithdrawals...)
	e.ExternalWithdrawals = append(e.ExternalWithdrawals, ae.ExternalWithdrawals...)
	e.WithdrawalConfirmations = append(e.WithdrawalConfirmations, ae.WithdrawalConfirmations...)
}

type BlockEvents struct {
	Events
	Block *ShardBlock // TODO: remove
}

type InternalWithdrawalTask struct {
	From        Address
	SubwalletID uint32
	Lt          uint64
	Currency    string
}

type TotalIncome struct {
	Deposit  Address
	Amount   Coins
	Currency string
}

type Coins = decimal.Decimal

func NewCoins(int *big.Int) Coins {
	return decimal.NewFromBigInt(int, 0)
}

func ZeroCoins() Coins {
	return decimal.New(0, 0)
}

// ShardID type copied from https://github.com/tonkeeper/tongo/blob/master/shards.go
//type ShardID struct {
//	// TODO: or use tongo ShardID type instead
//	prefix int64
//	mask   int64
//}

//func ParseShardID(m int64) (ShardID, error) {
//	if m == 0 {
//		return ShardID{}, errors.New("at least one non-zero bit required in shard id")
//	}
//	trailingZeros := bits.TrailingZeros64(uint64(m))
//	return ShardID{
//		prefix: m ^ (1 << trailingZeros),
//		mask:   -1 << (trailingZeros + 1),
//	}, nil
//}

//func ShardIdFromAddress(a Address, shardPrefixLen int) ShardID {
//	return ShardID{
//		prefix: int64(binary.BigEndian.Uint64(a[:8])),
//		mask:   -1 << (64 - shardPrefixLen),
//	}
//	// TODO: check for correctness with full prefix
//}
//
//func (s ShardID) MatchAddress(a Address) bool {
//	aPrefix := binary.BigEndian.Uint64(a[:8])
//	return (int64(aPrefix) & s.mask) == s.prefix
//}
//
//func (s ShardID) MatchBlockID(block *ton.BlockIDExt) bool {
//	sub, err := ParseShardID(block.Shard)
//	if err != nil {
//		// TODO: log error
//		return false
//	}
//	if bits.TrailingZeros64(uint64(s.mask)) < bits.TrailingZeros64(uint64(sub.mask)) {
//		return s.prefix&sub.mask == sub.prefix
//	}
//	return sub.prefix&s.mask == s.prefix
//}

// ShardBlock
// Block data for a specific shard mask attribute.
type ShardBlock struct {
	tongo.BlockIDExt
	//IsMaster bool
	GenUtime uint32
	//StartLt  uint64
	//EndLt    uint64
	Transactions []*tlb.Transaction
	Parents      []tongo.BlockIDExt
}

//func (s ShardBlockHeader) MatchParentBlockByAddress(a Address) (*ton.BlockIDExt, error) {
//	for _, p := range s.Parents {
//		shardID, err := ParseShardID(p.Shard)
//		if err != nil {
//			return nil, err
//		}
//		if shardID.MatchAddress(a) {
//			return p, nil
//		}
//	}
//	return nil, fmt.Errorf("must be at least one suitable block for this address")
//}
