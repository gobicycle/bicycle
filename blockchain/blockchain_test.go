package blockchain

import (
	"bytes"
	"context"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/tonkeeper/tongo/boc"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton/jetton"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"
)

var (
	jettonMasterAddress, _ = address.ParseAddr("kQCKt2WPGX-fh0cIAz38Ljd_OKQjoZE_cqk7QrYGsNP6wfP0") // TGR in Testnet
	activeAccount, _       = address.ParseAddr("kQCOSEttz9aEGXkjd1h_NJsQqOca3T-Pld5zSIPHcYZIxsyf")
	notActiveAccount, _    = address.ParseAddr("kQAkRRJ1RiViVHY2UmUhWCFjdiZBeEYnhkhxI1JTJFNUNG9v")
)

func init() {
	conf, err := boc.DeserializeBocBase64(config.TestnetConfig)
	if err != nil {
		panic(err)
	}
	config.Config.BlockchainConfig = conf[0]
}

func connect(t *testing.T) *Connection {
	server := os.Getenv("SERVER")
	if server == "" {
		t.Fatal("empty server var")
	}
	key := os.Getenv("KEY")
	if server == "" {
		t.Fatal("empty key var")
	}
	c, err := NewConnection(server, key)
	if err != nil {
		t.Fatal("connections err: ", err)
	}
	return c
}

func getSeed() string {
	seed := os.Getenv("SEED")
	if seed == "" {
		panic("empty seed")
	}
	return seed
}

func Test_NewConnection(t *testing.T) {
	connect(t)
}

func Test_GenerateDefaultWallet(t *testing.T) {
	c := connect(t)
	seed := getSeed()
	hlWallet, shard, id, err := c.GenerateDefaultWallet(seed, false)
	if err != nil {
		t.Fatal("gen default wallet err: ", err)
	}
	if hlWallet.Address().Data()[0] != shard {
		t.Fatal("invalid shard")
	}
	if id != wallet.DefaultSubwallet {
		t.Fatal("invalid subwallet ID")
	}
	w, shard, id, err := c.GenerateDefaultWallet(seed, true)
	if err != nil {
		t.Fatal("gen default wallet err: ", err)
	}
	if w.Address().Data()[0] != shard {
		t.Fatal("invalid shard")
	}
	if id != wallet.DefaultSubwallet {
		t.Fatal("invalid subwallet ID")
	}
}

func Test_GenerateSubWallet(t *testing.T) {
	c := connect(t)
	seed := getSeed()
	for i := 0; i < 10; i++ {
		shard := byte(rand.Intn(255))
		startSubWalletID := rand.Uint32()
		subWallet, subWalletID, err := c.GenerateSubWallet(seed, shard, startSubWalletID)
		if err != nil {
			t.Fatal("gen sub wallet err: ", err)
		}
		if subWalletID <= startSubWalletID {
			t.Fatal("invalid subwallet ID")
		}
		if subWallet.Address().Data()[0] != shard {
			t.Fatal("invalid shard")
		}
	}
}

func Test_GetJettonWalletAddress(t *testing.T) {
	c := connect(t)
	seed := getSeed()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	owner, _, _, err := c.GenerateDefaultWallet(seed, true)
	if err != nil {
		t.Fatal("gen owner wallet err: ", err)
	}
	jettonWalletAddr, err := c.GetJettonWalletAddress(ctx, owner.Address(), jettonMasterAddress)
	if err != nil {
		t.Fatal("get jetton wallet address err: ", err)
	}
	master := jetton.NewJettonMasterClient(c.client, jettonMasterAddress)
	jettonWallet, err := master.GetJettonWallet(ctx, owner.Address())
	if err != nil {
		t.Fatal("get jetton wallet address by tonutils method err: ", err)
	}
	if !bytes.Equal(jettonWallet.Address().Data(), jettonWalletAddr.Data()) ||
		jettonWallet.Address().Workchain() != jettonWalletAddr.Workchain() {
		t.Fatal("invalid jetton wallet address")
	}
}

func Test_GenerateJettonWalletAddressForProxy(t *testing.T) {
	c := connect(t)
	seed := getSeed()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	owner, _, _, err := c.GenerateDefaultWallet(seed, true)
	if err != nil {
		t.Fatal("gen owner wallet err: ", err)
	}
	master := jetton.NewJettonMasterClient(c.client, jettonMasterAddress)
	for i := 0; i < 10; i++ {
		shard := byte(rand.Intn(255))
		startSubWalletID := rand.Uint32()
		proxy, jettonWalletAddr, err := c.GenerateDepositJettonWalletForProxy(ctx, shard, owner.Address(), jettonMasterAddress, startSubWalletID)
		if err != nil {
			t.Fatal("gen sub wallet err: ", err)
		}
		if proxy == nil {
			t.Fatal("nil owner wallet")
		}
		if jettonWalletAddr == nil {
			t.Fatal("nil jetton wallet address")
		}
		if proxy.SubwalletID <= startSubWalletID {
			t.Fatal("invalid subwallet ID")
		}
		if jettonWalletAddr.Data()[0] != shard {
			t.Fatal("invalid shard")
		}
		jettonWallet, err := master.GetJettonWallet(ctx, proxy.Address())
		if err != nil {
			t.Fatal("get jetton wallet address by tonutils method err: ", err)
		}
		if jettonWallet.Address().String() != jettonWalletAddr.String() {
			t.Fatal("invalid jetton wallet address")
		}
	}
}

func Test_GetJettonBalance(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	block, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		t.Fatal("get current masterchain err: ", err)
	}
	coreAddr1 := core.AddressMustFromTonutilsAddress(activeAccount)
	coreAddr2 := core.AddressMustFromTonutilsAddress(notActiveAccount)
	b1, err := c.GetJettonBalance(ctx, coreAddr1, block)
	if err != nil {
		t.Fatal("get balance: ", err)
	}
	if b1.Cmp(big.NewInt(0)) != 1 {
		t.Fatal("empty balance: ", err)
	}
	b2, err := c.GetJettonBalance(ctx, coreAddr2, block)
	if err != nil {
		t.Fatal("get balance: ", err)
	}
	if b2.Cmp(big.NewInt(0)) != 0 {
		t.Fatal("not empty balance: ", err)
	}
}

func Test_GetAccountCurrentState(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	b1, st1, err := c.GetAccountCurrentState(ctx, activeAccount)
	if err != nil {
		t.Fatal("get acc current state err: ", err)
	}
	if b1.Cmp(big.NewInt(0)) != 1 || st1 != tlb.AccountStatusActive {
		t.Fatal("acc not active")
	}
	b2, st2, err := c.GetAccountCurrentState(ctx, notActiveAccount)
	if err != nil {
		t.Fatal("get acc current state err: ", err)
	}
	if b2.Cmp(big.NewInt(0)) != 0 || st2 != tlb.AccountStatusNonExist {
		t.Fatal("acc active")
	}
}

func Test_DeployTonWallet(t *testing.T) {
	c := connect(t)
	seed := getSeed()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	amount := tlb.FromNanoTONU(100_000_000)
	mainWallet, _, _, err := c.GenerateDefaultWallet(seed, true)
	if err != nil {
		t.Fatal("gen main wallet err: ", err)
	}
	b, st, err := c.GetAccountCurrentState(ctx, mainWallet.Address())
	if err != nil {
		t.Fatal("get acc current state err: ", err)
	}
	if b.Cmp(amount.NanoTON()) != 1 || st != tlb.AccountStatusActive {
		t.Fatal("wallet not active")
	}
	newWallet, err := mainWallet.GetSubwallet(3567745334)
	if err != nil {
		t.Fatal("gen new wallet err: ", err)
	}
	//fmt.Printf("Main wallet: %v\n", mainWallet.Address().String())
	//fmt.Printf("New wallet: %v\n", newWallet.Address().String())
	_, st, err = c.GetAccountCurrentState(ctx, newWallet.Address())
	if err != nil {
		t.Fatal("get acc current state err: ", err)
	}
	if st != tlb.AccountStatusNonExist {
		t.Fatal("wallet not empty")
	}
	err = mainWallet.TransferNoBounce(
		ctx,
		newWallet.Address(),
		amount,
		"",
		true,
	)
	if err != nil {
		t.Fatal("transfer err: ", err)
	}
	err = c.WaitStatus(ctx, newWallet.Address(), tlb.AccountStatusUninit)
	if err != nil {
		t.Fatal("wait uninit err: ", err)
	}
	err = c.DeployTonWallet(ctx, newWallet)
	if err != nil {
		t.Fatal("deploy new wallet err: ", err)
	}
	err = newWallet.Send(ctx, &wallet.Message{
		Mode: 128 + 32, // 128 + 32 send all and destroy
		InternalMessage: &tlb.InternalMessage{
			IHRDisabled: true,
			Bounce:      false,
			DstAddr:     mainWallet.Address(),
			Amount:      tlb.FromNanoTONU(0),
			Body:        nil,
		},
	}, false)
	if err != nil {
		t.Fatal("send withdrawal err: ", err)
	}
	err = c.WaitStatus(ctx, newWallet.Address(), tlb.AccountStatusNonExist)
	if err != nil {
		t.Fatal("wait empty err: ", err)
	}
}

func Test_GetTransactionIDsFromBlock(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	masterID, err := c.client.CurrentMasterchainInfo(ctx)
	if err != nil {
		t.Fatal("get last block err: ", err)
	}
	_, err = c.GetTransactionIDsFromBlock(ctx, masterID)
	if err != nil {
		t.Fatal("get tx ids err: ", err)
	}
}

func Test_GetTransactionFromBlock(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	for {
		masterID, err := c.client.CurrentMasterchainInfo(ctx)
		if err != nil {
			t.Fatal("get last block err: ", err)
		}
		txIDs, err := c.GetTransactionIDsFromBlock(ctx, masterID)
		if err != nil {
			t.Fatal("get tx ids err: ", err)
		}
		if len(txIDs) > 0 {
			tx, err := c.GetTransactionFromBlock(ctx, masterID, txIDs[0])
			if err != nil {
				t.Fatal("get tx err: ", err)
			}
			if tx == nil {
				t.Fatal("nil tx")
			}
			break
		}
	}
}

func Test_CheckTime(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	res, err := c.CheckTime(ctx, time.Second*0)
	if err != nil {
		t.Fatal("check time err: ", err)
	}
	if res == true {
		t.Fatal("time diff can not be 0")
	}
	res, err = c.CheckTime(ctx, time.Hour*1000)
	if err != nil {
		t.Fatal("check time err: ", err)
	}
	if res == false {
		t.Fatal("failed for extra large cutoff")
	}
}

func Test_WaitStatus(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*7)
	defer cancel()
	err := c.WaitStatus(ctx, activeAccount, tlb.AccountStatusActive)
	if err != nil {
		t.Fatal("wait status err: ", err)
	}
	err = c.WaitStatus(ctx, activeAccount, tlb.AccountStatusNonExist)
	if err == nil {
		t.Fatal("must be timeout error")
	}
}

func Test_GetAccount(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	for i := 0; i < 20; i++ {
		b, err := c.client.GetMasterchainInfo(ctx)
		if err != nil {
			t.Fatal("get masterchain info err: ", err)
		}
		_, err = c.GetAccount(ctx, b, activeAccount)
		if err != nil {
			t.Fatal("get account err: ", err)
		}
	}
}

func Test_RunGetMethod(t *testing.T) {
	c := connect(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()
	for i := 0; i < 20; i++ {
		b, err := c.client.GetMasterchainInfo(ctx)
		if err != nil {
			t.Fatal("get masterchain info err: ", err)
		}
		_, err = c.RunGetMethod(ctx, b, jettonMasterAddress, "get_jetton_data")
		if err != nil {
			t.Fatal("run get method err: ", err)
		}
	}
}

func Test_NextBlock(t *testing.T) {
	c := connect(t)
	var shard byte = 123
	st := NewShardTracker(shard, nil, c)
	for i := 0; i < 5; i++ {
		h, _, err := st.NextBlock()
		if err != nil {
			t.Fatal("get next block err: ", err)
		}
		if !isInShard(uint64(h.Shard), shard) {
			t.Fatal("next block not in shard")
		}
	}
}

func Test_Stop(t *testing.T) {
	c := connect(t)
	st := NewShardTracker(123, nil, c)
	for i := 0; i < 2; i++ {
		_, _, err := st.NextBlock()
		if err != nil {
			t.Fatal("get next block err: ", err)
		}
	}
	st.Stop()
	_, flag, err := st.NextBlock()
	if err != nil {
		t.Fatal("get next block err: ", err)
	}
	if !flag {
		t.Fatal("no shutdown flag")
	}
}
