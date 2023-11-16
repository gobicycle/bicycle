package blockchain

import (
	"context"
	"fmt"
	core "github.com/gobicycle/bicycle/internal/core"
	log "github.com/sirupsen/logrus"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	tongoConfig "github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/liteapi"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/ton"
	"github.com/tonkeeper/tongo/wallet"
	"math/big"
	"strings"
	"time"
)

const ErrBlockNotApplied = "block is not applied"

type Connection struct {
	client *liteapi.Client
	//	blockchainConfig string
}

//type contract struct {
//	Address tongo.AccountID
//	Code    string
//	Data    string
//}

// NewConnection creates new Blockchain connection
func NewConnection(ctx context.Context, cfg []tongoConfig.LiteServer) (*Connection, error) {
	// TODO: use all config.Blockchain struct!
	// TODO: parse in config
	//liteserver := tongoConfig.LiteServer{cfg.Blockchain.GlobalLiteservers}
	client, err := liteapi.NewClient(liteapi.WithLiteServers(cfg))
	if err != nil {
		return nil, fmt.Errorf("liteapi creating err: %v", err.Error())
	}

	// TODO: or get config before emulation?
	configParams, err := client.GetConfigAll(ctx, liteapi.NeedStateRoot) // TODO: clarify flag!
	if err != nil {
		return nil, err
	}

	// TODO: save config as cell (careful with pointers)

	configCell := boc.NewCell()
	err = tlb.Marshal(configCell, configParams)
	if err != nil {
		return nil, err
	}
	//blockchainConfig, err := configCell.ToBocBase64()
	//if err != nil {
	//	return nil, err
	//}

	return &Connection{client: client}, nil
}

// GenerateDefaultWallet generates HighloadV2R2 or V3R2 TON wallet with
// default subwallet_id and returns wallet, shard and subwalletID
func (c *Connection) GenerateDefaultWallet(seed string, isHighload bool) (
	w *wallet.Wallet,
	subwalletID uint32, err error,
) {
	pk, err := wallet.SeedToPrivateKey(seed)
	if err != nil {
		return nil, 0, err
	}

	subWalletID := wallet.DefaultSubWallet

	if isHighload {
		hw, err := wallet.New(pk, wallet.HighLoadV2R2, core.DefaultWorkchainID, &subWalletID, c.client)
		w = &hw
		if err != nil {
			return nil, 0, err
		}
		//w, err = wallet.FromSeed(c, words, wallet.HighloadV2R2)
	} else {
		ow, err := wallet.New(pk, wallet.V3R2, core.DefaultWorkchainID, &subWalletID, c.client)
		w = &ow
		if err != nil {
			return nil, 0, err
		}
		//w, err = wallet.FromSeed(c, words, wallet.V3)
	}
	return w, uint32(subWalletID), nil
}

// GetJettonWalletAddress generates jetton wallet address from owner and jetton master addresses
func (c *Connection) GetJettonWalletAddress(
	ctx context.Context,
	owner tongo.AccountID,
	jettonMaster tongo.AccountID,
) (tongo.AccountID, error) {
	return c.client.GetJettonWallet(ctx, jettonMaster, owner)
}

//func (c *Connection) getContract(ctx context.Context, addr tongo.AccountID) (contract, error) {
//
//	as, err := c.client.GetAccountState(ctx, addr)
//	if err != nil {
//		return contract{}, err
//	}
//
//	//ai, err := tongo.GetAccountInfo(sa.Account)
//	//if err != nil {
//	//	return contract{}, err
//	//}
//
//	if as.Account.Status() != tlb.AccountActive {
//		return contract{}, fmt.Errorf("account is not active")
//	} else if !as.Account.Account.Storage.State.AccountActive.StateInit.Code.Exists {
//		return contract{}, fmt.Errorf("empty account code")
//	} else if !as.Account.Account.Storage.State.AccountActive.StateInit.Data.Exists {
//		return contract{}, fmt.Errorf("empty account data")
//	}
//
//	code, err := as.Account.Account.Storage.State.AccountActive.StateInit.Code.Value.Value.ToBocBase64()
//	if err != nil {
//		return contract{}, err
//	}
//
//	data, err := as.Account.Account.Storage.State.AccountActive.StateInit.Data.Value.Value.ToBocBase64()
//	if err != nil {
//		return contract{}, err
//	}
//
//	return contract{
//		Address: addr,
//		Code:    code,
//		Data:    data,
//	}, nil
//}

// GetJettonBalance
// Returns jetton balance for custom block in basic units
func (c *Connection) GetJettonBalance(ctx context.Context, jettonWallet tongo.AccountID, blockID tongo.BlockIDExt) (*big.Int, error) {
	return c.client.WithBlock(blockID).GetJettonBalance(ctx, jettonWallet)
}

// GetLastJettonBalance
// Returns jetton balance for last block in basic units
func (c *Connection) GetLastJettonBalance(ctx context.Context, jettonWallet tongo.AccountID) (*big.Int, error) {
	return c.client.GetJettonBalance(ctx, jettonWallet)
}

// GetAccountCurrentState
// Returns TON balance in nanoTONs and account status
func (c *Connection) GetAccountCurrentState(ctx context.Context, address tongo.AccountID) (uint64, tlb.AccountStatus, error) {

	as, err := c.client.GetAccountState(ctx, address)
	if err != nil {
		return 0, "", err
	}

	//ai, err := tongo.GetAccountInfo(as.Account)
	//if err != nil {
	//	return 0, "", err
	//}

	if as.Account.Status() == tlb.AccountNone {
		return 0, tlb.AccountNone, nil
	}

	return uint64(as.Account.Account.Storage.Balance.Grams), as.Account.Status(), nil // TODO: use Grams

	//account, err := c.GetAccount(ctx, masterID, address)
	//if err != nil {
	//	return nil, "", err
	//}
	//if !account.IsActive {
	//	return big.NewInt(0), tlb.AccountStatusNonExist, nil
	//}
	//return account.State.Balance.NanoTON(), account.State.Status, nil
}

// DeployTonWallet
// Deploys wallet contract and wait its activation
func (c *Connection) DeployTonWallet(ctx context.Context, w *wallet.Wallet) error {
	addr := w.GetAddress()
	balance, status, err := c.GetAccountCurrentState(ctx, addr)
	if err != nil {
		return err
	}
	if balance == 0 {
		return fmt.Errorf("empty balance")
	}
	if status != tlb.AccountActive {
		err := w.Send(ctx, wallet.SimpleTransfer{
			Amount:     0,
			Address:    addr,
			Bounceable: false,
		})
		//err = wallet.TransferNoBounce(ctx, wallet.Address(), tlb.FromNanoTONU(0), "")
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	return c.WaitStatus(ctx, addr, tlb.AccountActive)
}

// lookupMasterchainBlock
// Try to find masterchain block with retry. Returns error if context timeout is exceeded.
// Context must be with timeout to avoid blocking!
func (c *Connection) lookupMasterchainBlock(ctx context.Context, seqno uint32) (*tongo.BlockIDExt, error) {
	id := tongo.BlockID{
		Workchain: -1,
		Shard:     -9223372036854775808,
		Seqno:     seqno,
	}

	// TODO: replace retry with waitblock
	for {
		select {
		case <-ctx.Done():
			return nil, core.ErrTimeoutExceeded // TODO: maybe remove
		default:
			idExt, _, err := c.client.LookupBlock(ctx, id, 1, nil, nil) // TODO: check mode
			if err != nil && isBlockNotReadyError(err) {
				time.Sleep(time.Millisecond * 200)
			} else if err != nil {
				return nil, err
			}
			return &idExt, nil
		}
	}
}

func isBlockNotReadyError(err error) bool {
	if strings.Contains(err.Error(), "ltdb: block not found") {
		return true
	}
	if strings.Contains(err.Error(), "block is not applied") {
		return true
	}
	return false
}

// GetTransactionIDsFromBlock
// Gets all transactions IDs from custom block
//func (c *Connection) GetTransactionIDsFromBlock(ctx context.Context, blockID *ton.BlockIDExt) ([]ton.TransactionShortInfo, error) {
//	var (
//		txIDList []ton.TransactionShortInfo
//		after    *ton.TransactionID3
//		next     = true
//	)
//	for next {
//		fetchedIDs, more, err := c.client.GetBlockTransactionsV2(ctx, blockID, 256, after)
//		if err != nil {
//			return nil, err
//		}
//		txIDList = append(txIDList, fetchedIDs...)
//		next = more
//		if more {
//			// set load offset for next query (pagination)
//			after = fetchedIDs[len(fetchedIDs)-1].ID3()
//		}
//	}
//	// sort by LT
//	sort.Slice(txIDList, func(i, j int) bool {
//		return txIDList[i].LT < txIDList[j].LT
//	})
//	return txIDList, nil
//}

// GetTransactionFromBlock
// Gets transaction from block
//func (c *Connection) GetTransactionFromBlock(ctx context.Context, blockID *ton.BlockIDExt, txID liteclient.LiteServerTransactionIdC) (*tongo.Transaction, error) {
//	// TODO: fix
//	tx, err := c.client.GetOneTransactionFromBlock(ctx, txID.Account, blockID, txID.Lt)
//	if err != nil {
//		return nil, err
//	}
//	return &tx, nil
//	//tx, err := c.client.GetTransaction(ctx, blockID, address.NewAddress(0, byte(blockID.Workchain), txID.Account), txID.LT)
//}

func (c *Connection) getCurrentNodeTime(ctx context.Context) (*time.Time, error) {
	t, err := c.client.GetTime(ctx)
	if err != nil {
		return nil, err
	}
	res := time.Unix(int64(t), 0)
	return &res, nil
}

// CheckTime
// Checks time diff between node and local time. Due to the fact that the request to the node takes time,
// the local time is defined as the average between the beginning and end of the request.
// Returns true if time diff < cutoff.
func (c *Connection) CheckTime(ctx context.Context, cutoff time.Duration) (bool, error) {
	prevTime := time.Now()
	nodeTime, err := c.getCurrentNodeTime(ctx)
	if err != nil {
		return false, err
	}
	nextTime := time.Now()
	midTime := prevTime.Add(nextTime.Sub(prevTime) / 2)
	nodeTimeShift := midTime.Sub(*nodeTime)
	log.Infof("Service-Node time diff: %v", nodeTimeShift)
	if nodeTimeShift > cutoff || nodeTimeShift < -cutoff {
		return false, nil
	}
	return true, nil
}

// WaitStatus
// Waits custom status for account. Returns error if context timeout is exceeded.
// Context must be with timeout to avoid blocking!
func (c *Connection) WaitStatus(ctx context.Context, addr tongo.AccountID, status tlb.AccountStatus) error {
	for {
		select {
		case <-ctx.Done():
			return core.ErrTimeoutExceeded
		default:
			_, st, err := c.GetAccountCurrentState(ctx, addr)
			if err != nil {
				return err
			}
			if st == status {
				return nil
			}
			time.Sleep(time.Millisecond * 200)
		}
	}
}

// tonutils TonAPI interface methods

// GetAccount
// The method is being redefined for more stable operation.
// Gets account from prev block if impossible to get it from current block. Be careful with diff calculation between blocks.
// TODO: remove and test
//func (c *Connection) GetAccount(ctx context.Context, block *ton.BlockIDExt, addr *address.Address) (*tlb.Account, error) {
//	res, err := c.client.GetAccount(ctx, block, addr)
//	if err != nil && strings.Contains(err.Error(), ErrBlockNotApplied) {
//		prevBlock, err := c.client.LookupBlock(ctx, block.Workchain, block.Shard, block.SeqNo-1)
//		if err != nil {
//			return nil, err
//		}
//		return c.client.GetAccount(ctx, prevBlock, addr)
//	}
//	return res, err
//}

//func (c *Connection) SendExternalMessage(ctx context.Context, msg *tlb.ExternalMessage) error {
//	return c.client.SendExternalMessage(ctx, msg)
//}

// RunGetMethod
// The method is being redefined for more stable operation
// Wait until BlockIsApplied. Use context with  timeout.
// TODO: check without this method
//func (c *Connection) RunGetMethod(ctx context.Context, block *ton.BlockIDExt, addr *address.Address, method string, params ...any) (*ton.ExecutionResult, error) {
//	for {
//		select {
//		case <-ctx.Done():
//			return nil, core.ErrTimeoutExceeded
//		default:
//			res, err := c.client.RunGetMethod(ctx, block, addr, method, params...)
//			if err != nil && strings.Contains(err.Error(), ErrBlockNotApplied) {
//				time.Sleep(time.Millisecond * 200)
//				continue
//			}
//			return res, err
//		}
//	}
//}

//func (c *Connection) ListTransactions(ctx context.Context, addr *address.Address, num uint32, lt uint64, txHash []byte) ([]*tlb.Transaction, error) {
//	return c.client.ListTransactions(ctx, addr, num, lt, txHash)
//}
//
//func (c *Connection) WaitNextMasterBlock(ctx context.Context, master *ton.BlockIDExt) (*ton.BlockIDExt, error) {
//	return c.client.WaitNextMasterBlock(ctx, master)
//}
//
//func (c *Connection) Client() ton.LiteClient {
//	return c.client.Client()
//}
//
//func (c *Connection) CurrentMasterchainInfo(ctx context.Context) (*ton.BlockIDExt, error) {
//	return c.client.CurrentMasterchainInfo(ctx)
//}
//
//func (c *Connection) GetMasterchainInfo(ctx context.Context) (*ton.BlockIDExt, error) {
//	return c.client.GetMasterchainInfo(ctx)
//}
//
//func (c *Connection) WaitForBlock(seqno uint32) ton.APIClientWaiter {
//	return c.client.WaitForBlock(seqno)
//}

//func (c *Connection) RunSmcMethod(ctx context.Context, accountID ton.AccountID, method string, params tlb.VmStack) (uint32, tlb.VmStack, error) {
//	return c.client.RunSmcMethod(ctx, accountID, method, params)
//}

func (c *Connection) RunSmcMethodByID(ctx context.Context, accountID ton.AccountID, methodID int, params tlb.VmStack) (uint32, tlb.VmStack, error) {
	return c.client.RunSmcMethodByID(ctx, accountID, methodID, params)
}
