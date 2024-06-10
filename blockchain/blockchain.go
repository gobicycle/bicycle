package blockchain

import (
	"context"
	"errors"
	"fmt"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	log "github.com/sirupsen/logrus"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	tongoTlb "github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/tvm"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/liteclient"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"github.com/xssnick/tonutils-go/ton/dns"
	"github.com/xssnick/tonutils-go/ton/jetton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

type Connection struct {
	client   ton.APIClientWrapped
	resolver *dns.Client
}

func (c *Connection) WaitForBlock(seqno uint32) ton.APIClientWrapped {
	return c.client.WaitForBlock(seqno)
}

func (c *Connection) FindLastTransactionByInMsgHash(ctx context.Context, addr *address.Address, msgHash []byte, maxTxNumToScan ...int) (*tlb.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (c *Connection) FindLastTransactionByOutMsgHash(ctx context.Context, addr *address.Address, msgHash []byte, maxTxNumToScan ...int) (*tlb.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

type contract struct {
	Address tongo.AccountID
	Code    *boc.Cell
	Data    *boc.Cell
}

// NewConnection creates new Blockchain connection
func NewConnection(addr, key string) (*Connection, error) {

	client := liteclient.NewConnectionPool()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	err := client.AddConnection(ctx, addr, key)
	if err != nil {
		return nil, fmt.Errorf("connection err: %v", err.Error())
	}

	var wrappedClient ton.APIClientWrapped

	if config.Config.ProofCheckEnabled {

		if config.Config.NetworkConfigUrl == "" {
			return nil, fmt.Errorf("empty network config URL")
		}

		cfg, err := liteclient.GetConfigFromUrl(ctx, config.Config.NetworkConfigUrl)
		if err != nil {
			return nil, fmt.Errorf("get network config from url err: %s", err.Error())
		}

		wrappedClient = ton.NewAPIClient(client, ton.ProofCheckPolicySecure).WithRetry()
		wrappedClient.SetTrustedBlockFromConfig(cfg)

		log.Infof("Fetching and checking proofs since config init block ...")
		_, err = wrappedClient.CurrentMasterchainInfo(ctx) // we fetch block just to trigger chain proof check
		if err != nil {
			return nil, fmt.Errorf("get masterchain info err: %s", err.Error())
		}
		log.Infof("Proof checks are completed")

	} else {
		wrappedClient = ton.NewAPIClient(client, ton.ProofCheckPolicyUnsafe).WithRetry()
	}

	root, err := dns.RootContractAddr(wrappedClient)
	if err != nil {
		return nil, fmt.Errorf("get DNS root contract err: %s", err.Error())
	}

	resolver := dns.NewDNSClient(wrappedClient, root)

	return &Connection{
		client:   wrappedClient,
		resolver: resolver,
	}, nil
}

// GenerateDefaultWallet generates HighloadV2R2 or V3R2 TON wallet with
// default subwallet_id and returns wallet, shard and subwalletID
func (c *Connection) GenerateDefaultWallet(seed string, isHighload bool) (
	w *wallet.Wallet,
	shard byte,
	subwalletID uint32, err error,
) {
	words := strings.Split(seed, " ")
	if isHighload {
		w, err = wallet.FromSeed(c, words, wallet.HighloadV2R2)
	} else {
		w, err = wallet.FromSeed(c, words, wallet.V3)
	}
	if err != nil {
		return nil, 0, 0, err
	}
	return w, w.Address().Data()[0], uint32(wallet.DefaultSubwallet), nil
}

// GenerateSubWallet generates subwallet for custom shard and
// subwallet_id >= startSubWalletId and returns wallet and new subwallet_id
func (c *Connection) GenerateSubWallet(seed string, shard byte, startSubWalletID uint32) (*wallet.Wallet, uint32, error) {
	words := strings.Split(seed, " ")
	basic, err := wallet.FromSeed(c, words, wallet.V3)
	if err != nil {
		return nil, 0, err
	}
	for id := startSubWalletID; id < math.MaxUint32; id++ {
		subWallet, err := basic.GetSubwallet(id)
		if err != nil {
			return nil, 0, err
		}
		addr, err := core.AddressFromTonutilsAddress(subWallet.Address())
		if err != nil {
			return nil, 0, err
		}
		if inShard(addr, shard) {
			return subWallet, id, nil
		}
	}
	return nil, 0, fmt.Errorf("subwallet not found")
}

// GetJettonWalletAddress generates jetton wallet address from owner and jetton master addresses
func (c *Connection) GetJettonWalletAddress(
	ctx context.Context,
	owner *address.Address,
	jettonMaster *address.Address,
) (*address.Address, error) {
	contr, err := c.getContract(ctx, jettonMaster)
	if err != nil {
		return nil, err
	}
	emulator, err := newEmulator(contr.Code, contr.Data)
	if err != nil {
		return nil, err
	}
	addr, err := getJettonWalletAddressByTVM(owner, contr.Address, emulator)
	if err != nil {
		return nil, err
	}
	res := addr.ToTonutilsAddressStd(0)
	res.SetTestnetOnly(config.Config.Testnet)
	return res, nil
}

func (c *Connection) GetJettonBalanceByOwner(
	ctx context.Context,
	owner *address.Address,
	jettonMaster *address.Address,
) (*big.Int, error) {

	jettonMasterClient := jetton.NewJettonMasterClient(c.client, jettonMaster)

	jettonWalletClient, err := jettonMasterClient.GetJettonWallet(ctx, owner)
	if err != nil {
		return nil, err
	}

	return jettonWalletClient.GetBalance(ctx)
}

func (c *Connection) DnsResolveSmc(
	ctx context.Context,
	domainName string,
) (*address.Address, error) {

	// TODO: it is necessary to distinguish network errors from the impossibility of resolving
	domain, err := c.resolver.Resolve(ctx, domainName)
	if errors.Is(err, dns.ErrNoSuchRecord) {
		return nil, core.ErrNotFound
	} else if err != nil {
		return nil, err
	}

	smcAddr := domain.GetWalletRecord()
	// TODO: fix tonutils

	if smcAddr == nil {
		// not wallet
		return nil, core.ErrNotFound
	}

	return smcAddr, nil
}

// GenerateDepositJettonWalletForProxy
// Generates jetton wallet address for custom shard and proxy contract as owner with subwallet_id >= startSubWalletId
func (c *Connection) GenerateDepositJettonWalletForProxy(
	ctx context.Context,
	shard byte,
	proxyOwner, jettonMaster *address.Address,
	startSubWalletID uint32,
) (
	proxy *core.JettonProxy,
	addr *address.Address,
	err error,
) {
	contr, err := c.getContract(ctx, jettonMaster)
	if err != nil {
		return nil, nil, err
	}
	emulator, err := newEmulator(contr.Code, contr.Data)
	if err != nil {
		return nil, nil, err
	}

	for id := startSubWalletID; id < math.MaxUint32; id++ {
		proxy, err = core.NewJettonProxy(id, proxyOwner)
		if err != nil {
			return nil, nil, err
		}
		jettonWalletAddress, err := getJettonWalletAddressByTVM(proxy.Address(), contr.Address, emulator)
		if err != nil {
			return nil, nil, err
		}
		if inShard(jettonWalletAddress, shard) {
			addr = jettonWalletAddress.ToTonutilsAddressStd(0)
			addr.SetTestnetOnly(config.Config.Testnet)
			return proxy, addr, nil
		}
	}
	return nil, nil, fmt.Errorf("jetton wallet address not found")
}

func (c *Connection) getContract(ctx context.Context, addr *address.Address) (contract, error) {
	block, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return contract{}, err
	}
	account, err := c.GetAccount(ctx, block, addr)
	if err != nil {
		return contract{}, err
	}
	if account == nil || account.Code == nil || account.Data == nil {
		return contract{}, fmt.Errorf("empty account code or data")
	}
	accountID, err := tongo.ParseAccountID(addr.String())
	if err != nil {
		return contract{}, err
	}
	codeCell, err := boc.DeserializeBoc(account.Code.ToBOC())
	if err != nil {
		return contract{}, err
	}
	if len(codeCell) != 1 {
		return contract{}, fmt.Errorf("BOC must have only one root")
	}
	dataCell, err := boc.DeserializeBoc(account.Data.ToBOC())
	if err != nil {
		return contract{}, err
	}
	if len(dataCell) != 1 {
		return contract{}, fmt.Errorf("BOC must have only one root")
	}
	return contract{
		Address: accountID,
		Code:    codeCell[0],
		Data:    dataCell[0],
	}, nil
}

func getJettonWalletAddressByTVM(
	owner *address.Address,
	jettonMaster tongo.AccountID,
	emulator *tvm.Emulator,
) (core.Address, error) {
	ownerAccountID, err := tongo.ParseAccountID(owner.String())
	if err != nil {
		return core.Address{}, err
	}
	slice, err := tongoTlb.TlbStructToVmCellSlice(ownerAccountID.ToMsgAddress())
	if err != nil {
		return core.Address{}, err
	}

	code, result, err := emulator.RunSmcMethod(context.Background(), jettonMaster, "get_wallet_address",
		tongoTlb.VmStack{slice})
	if err != nil {
		return core.Address{}, err
	}
	if code != 0 || len(result) != 1 || result[0].SumType != "VmStkSlice" {
		return core.Address{}, fmt.Errorf("tvm execution failed")
	}

	var msgAddress tongoTlb.MsgAddress
	err = result[0].VmStkSlice.UnmarshalToTlbStruct(&msgAddress)
	if err != nil {
		return core.Address{}, err
	}
	if msgAddress.SumType != "AddrStd" {
		return core.Address{}, fmt.Errorf("not std jetton wallet address")
	}
	if msgAddress.AddrStd.WorkchainId != core.DefaultWorkchain {
		return core.Address{}, fmt.Errorf("not default workchain for jetton wallet address")
	}
	return core.Address(msgAddress.AddrStd.Address), nil
}

func newEmulator(code, data *boc.Cell) (*tvm.Emulator, error) {
	emulator, err := tvm.NewEmulator(code, data, config.Config.BlockchainConfig, tvm.WithBalance(1_000_000_000))
	if err != nil {
		return nil, err
	}
	// TODO: try tvm.WithLazyC7Optimization()
	return emulator, nil
}

// GetJettonBalance
// Get method get_wallet_data() returns (int balance, slice owner, slice jetton, cell jetton_wallet_code)
// Returns jetton balance for custom block in basic units
func (c *Connection) GetJettonBalance(ctx context.Context, address core.Address, blockID *ton.BlockIDExt) (*big.Int, error) {
	jettonWallet := address.ToTonutilsAddressStd(0)
	stack, err := c.RunGetMethod(ctx, blockID, jettonWallet, "get_wallet_data")
	if err != nil {
		if strings.Contains(err.Error(), "contract is not initialized") {
			return big.NewInt(0), nil
		}
		return nil, fmt.Errorf("get wallet data error: %v", err)
	}
	res := stack.AsTuple()
	if len(res) != 4 {
		return nil, fmt.Errorf("invalid stack size")
	}
	switch res[0].(type) {
	case *big.Int:
		return res[0].(*big.Int), nil
	default:
		return nil, fmt.Errorf("invalid balance type")
	}
}

// GetLastJettonBalance
// Returns jetton balance for last block in basic units
func (c *Connection) GetLastJettonBalance(ctx context.Context, address *address.Address) (*big.Int, error) {
	masterID, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, err
	}
	addr, err := core.AddressFromTonutilsAddress(address)
	if err != nil {
		return nil, err
	}
	return c.GetJettonBalance(ctx, addr, masterID)
}

// GetAccountCurrentState
// Returns TON balance in nanoTONs and account status
func (c *Connection) GetAccountCurrentState(ctx context.Context, address *address.Address) (*big.Int, tlb.AccountStatus, error) {
	masterID, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		return nil, "", err
	}
	account, err := c.GetAccount(ctx, masterID, address)
	if err != nil {
		return nil, "", err
	}
	if !account.IsActive {
		return big.NewInt(0), tlb.AccountStatusNonExist, nil
	}
	return account.State.Balance.Nano(), account.State.Status, nil
}

// DeployTonWallet
// Deploys wallet contract and wait its activation
func (c *Connection) DeployTonWallet(ctx context.Context, wallet *wallet.Wallet) error {
	balance, status, err := c.GetAccountCurrentState(ctx, wallet.Address())
	if err != nil {
		return err
	}
	if balance.Cmp(big.NewInt(0)) == 0 {
		return fmt.Errorf("empty balance")
	}
	if status != tlb.AccountStatusActive {
		err = wallet.TransferNoBounce(ctx, wallet.Address(), tlb.FromNanoTONU(0), "")
		if err != nil {
			return err
		}
	} else {
		return nil
	}
	return c.WaitStatus(ctx, wallet.Address(), tlb.AccountStatusActive)
}

// GetTransactionIDsFromBlock
// Gets all transactions IDs from custom block
func (c *Connection) GetTransactionIDsFromBlock(ctx context.Context, blockID *ton.BlockIDExt) ([]ton.TransactionShortInfo, error) {
	var (
		txIDList []ton.TransactionShortInfo
		after    *ton.TransactionID3
		next     = true
	)
	for next {
		fetchedIDs, more, err := c.client.GetBlockTransactionsV2(ctx, blockID, 256, after)
		if err != nil {
			return nil, err
		}
		txIDList = append(txIDList, fetchedIDs...)
		next = more
		if more {
			// set load offset for next query (pagination)
			after = fetchedIDs[len(fetchedIDs)-1].ID3()
		}
	}
	// sort by LT
	sort.Slice(txIDList, func(i, j int) bool {
		return txIDList[i].LT < txIDList[j].LT
	})
	return txIDList, nil
}

// GetTransactionFromBlock
// Gets transaction from block
func (c *Connection) GetTransactionFromBlock(ctx context.Context, blockID *ton.BlockIDExt, txID ton.TransactionShortInfo) (*tlb.Transaction, error) {
	tx, err := c.client.GetTransaction(ctx, blockID, address.NewAddress(0, byte(blockID.Workchain), txID.Account), txID.LT)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func inShard(addr core.Address, shard byte) bool {
	return addr[0] == shard
}

func (c *Connection) getCurrentNodeTime(ctx context.Context) (time.Time, error) {
	t, err := c.client.GetTime(ctx)
	if err != nil {
		return time.Time{}, err
	}
	res := time.Unix(int64(t), 0)
	return res, nil
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
	nodeTimeShift := midTime.Sub(nodeTime)
	log.Infof("Service-Node time diff: %v", nodeTimeShift)
	if nodeTimeShift > cutoff || nodeTimeShift < -cutoff {
		return false, nil
	}
	return true, nil
}

// WaitStatus
// Waits custom status for account. Returns error if context timeout is exceeded.
// Context must be with timeout to avoid blocking!
func (c *Connection) WaitStatus(ctx context.Context, addr *address.Address, status tlb.AccountStatus) error {
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
func (c *Connection) GetAccount(ctx context.Context, block *ton.BlockIDExt, addr *address.Address) (*tlb.Account, error) {
	res, err := c.client.GetAccount(ctx, block, addr)
	if err != nil && isNotReadyError(err) {
		prevBlock, err := c.client.LookupBlock(ctx, block.Workchain, block.Shard, block.SeqNo-1)
		if err != nil {
			return nil, err
		}
		return c.client.GetAccount(ctx, prevBlock, addr)
	}
	return res, err
}

func (c *Connection) SendExternalMessage(ctx context.Context, msg *tlb.ExternalMessage) error {
	return c.client.SendExternalMessage(ctx, msg)
}

// RunGetMethod
// The method is being redefined for more stable operation
// Wait until BlockIsApplied. Use context with  timeout.
func (c *Connection) RunGetMethod(ctx context.Context, block *ton.BlockIDExt, addr *address.Address, method string, params ...any) (*ton.ExecutionResult, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, core.ErrTimeoutExceeded
		default:
			res, err := c.client.RunGetMethod(ctx, block, addr, method, params...)
			if err != nil && isNotReadyError(err) {
				time.Sleep(time.Millisecond * 200)
				continue
			}
			return res, err
		}
	}
}

func (c *Connection) ListTransactions(ctx context.Context, addr *address.Address, num uint32, lt uint64, txHash []byte) ([]*tlb.Transaction, error) {
	return c.client.ListTransactions(ctx, addr, num, lt, txHash)
}

func (c *Connection) Client() ton.LiteClient {
	return c.client.Client()
}

func (c *Connection) CurrentMasterchainInfo(ctx context.Context) (*ton.BlockIDExt, error) {
	return c.client.CurrentMasterchainInfo(ctx)
}

func (c *Connection) GetMasterchainInfo(ctx context.Context) (*ton.BlockIDExt, error) {
	return c.client.GetMasterchainInfo(ctx)
}
