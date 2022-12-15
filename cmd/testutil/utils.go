package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/gobicycle/bicycle/api"
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/gofrs/uuid"
	"github.com/shopspring/decimal"
	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/ton/wallet"
	"io"
	"math/big"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	client *http.Client
	url    string
	token  string
	userID string
}

type PayerProcessor struct {
	client       *Client
	deposits     []Deposit
	payer        *wallet.Wallet
	bcClient     *blockchain.Connection
	payerWallets map[string]Wallet
	hotWallets   map[string]Wallet
	mutex        sync.Mutex
}

type Deposit struct {
	Address      *address.Address
	JettonWallet *address.Address
	Currency     string
}

type Wallet struct {
	Address *address.Address
	Balance *big.Int
}

func NewClient(url, token, userID string) *Client {
	c := &Client{
		client: &http.Client{Timeout: 10 * time.Second},
		url:    url,
		token:  token,
		userID: userID,
	}
	return c
}

func (s *Client) GetAllAddresses() (api.GetAddressesResponse, error) {
	url := fmt.Sprintf("http://%s/v1/address/all?user_id=%s", s.url, s.userID)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return api.GetAddressesResponse{}, err
	}
	request.Header.Add("Authorization", "Bearer "+s.token)
	response, err := s.client.Do(request)
	if err != nil {
		return api.GetAddressesResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return api.GetAddressesResponse{}, fmt.Errorf("response status: %v", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return api.GetAddressesResponse{}, err
	}
	var res api.GetAddressesResponse
	err = json.Unmarshal(content, &res)
	if err != nil {
		return api.GetAddressesResponse{}, err
	}
	return res, nil
}

func (s *Client) SendWithdrawal(currency, destination string, amount int64) (api.WithdrawalResponse, error) {
	url := fmt.Sprintf("http://%s/v1/withdrawal/send", s.url)
	u, err := uuid.NewV4()
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	reqData := api.WithdrawalRequest{
		UserID:      s.userID,
		QueryID:     u.String(),
		Currency:    currency,
		Amount:      decimal.New(amount, 0),
		Destination: destination,
		Comment:     fmt.Sprintf("User uuid: %s", u.String()),
	}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Add("Authorization", "Bearer "+s.token)
	response, err := s.client.Do(request)
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return api.WithdrawalResponse{}, fmt.Errorf("response status: %v", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	var res api.WithdrawalResponse
	err = json.Unmarshal(content, &res)
	if err != nil {
		return api.WithdrawalResponse{}, err
	}
	return res, nil
}

func (s *Client) GetNewAddress(currency string) (string, error) {
	url := fmt.Sprintf("http://%s/v1/address/new", s.url)
	reqData := struct {
		UserID   string `json:"user_id"`
		Currency string `json:"currency"`
	}{
		UserID:   s.userID,
		Currency: currency,
	}
	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json; charset=UTF-8")
	request.Header.Add("Authorization", "Bearer "+s.token)
	response, err := s.client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("response status: %v", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return "", err
	}
	var res struct {
		Address string `json:"address"`
	}
	err = json.Unmarshal(content, &res)
	if err != nil {
		return "", err
	}
	return res.Address, nil
}

func (s *Client) GetWithdrawalStatus(id int64) (api.WithdrawalStatusResponse, error) {
	url := fmt.Sprintf("http://%s/v1/withdrawal/status?id=%v", s.url, id)
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return api.WithdrawalStatusResponse{}, err
	}
	request.Header.Add("Authorization", "Bearer "+s.token)
	response, err := s.client.Do(request)
	if err != nil {
		return api.WithdrawalStatusResponse{}, err
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return api.WithdrawalStatusResponse{}, fmt.Errorf("response status: %v", response.Status)
	}
	content, err := io.ReadAll(response.Body)
	if err != nil {
		return api.WithdrawalStatusResponse{}, err
	}
	var res api.WithdrawalStatusResponse
	err = json.Unmarshal(content, &res)
	if err != nil {
		return api.WithdrawalStatusResponse{}, err
	}
	return res, nil
}

func (s *Client) InitDeposits() (map[string][]string, error) {
	deposits := make(map[string][]string)
	addr, err := s.GetAllAddresses()
	if err != nil {
		return nil, err
	}
	if len(addr.Addresses) != 0 {
		for _, wa := range addr.Addresses {
			deposits[wa.Currency] = append(deposits[wa.Currency], wa.Address)
		}
		return deposits, nil
	}
	for i := 0; i < depositsQty; i++ {
		addr, err := s.GetNewAddress(core.TonSymbol)
		if err != nil {
			return nil, err
		}
		deposits[core.TonSymbol] = append(deposits[core.TonSymbol], addr)
	}
	for i := 0; i < depositsQty; i++ {
		for cur := range config.Config.Jettons {
			addr, err := s.GetNewAddress(cur)
			if err != nil {
				return nil, err
			}
			deposits[cur] = append(deposits[cur], addr)
		}
	}
	fmt.Println("deposits initialized")
	return deposits, nil
}

func NewPayerProcessor(
	client *Client,
	deposits map[string][]string,
	payer, hot *wallet.Wallet,
	bcClient *blockchain.Connection,
) *PayerProcessor {
	var dep []Deposit
	pWal := make(map[string]Wallet)
	hWal := make(map[string]Wallet)
	for cur, addresses := range deposits {
		if cur != core.TonSymbol {
			jetton := config.Config.Jettons[cur]
			jwAddr, err := bcClient.GetJettonWalletAddress(context.Background(), payer.Address(), jetton.Master)
			if err != nil {
				log.Fatalf("get jetton wallet error: %v", err)
			}
			jwHotAddr, err := bcClient.GetJettonWalletAddress(context.Background(), hot.Address(), jetton.Master)
			if err != nil {
				log.Fatalf("get hot jetton wallet error: %v", err)
			}
			pWal[cur] = Wallet{Address: jwAddr, Balance: big.NewInt(0)}
			hWal[cur] = Wallet{Address: jwHotAddr, Balance: big.NewInt(0)}

			for _, a := range addresses {
				addr, _ := address.ParseAddr(a)
				jwAddr, err := bcClient.GetJettonWalletAddress(context.Background(), addr, jetton.Master)
				if err != nil {
					log.Fatalf("get jetton wallet error: %v", err)
				}
				dep = append(dep, Deposit{Address: addr, Currency: cur, JettonWallet: jwAddr})
			}
		} else {
			for _, a := range addresses {
				addr, _ := address.ParseAddr(a)
				dep = append(dep, Deposit{Address: addr, Currency: cur})
			}
		}
	}
	pWal[core.TonSymbol] = Wallet{Address: payer.Address(), Balance: big.NewInt(0)}
	hWal[core.TonSymbol] = Wallet{Address: hot.Address(), Balance: big.NewInt(0)}
	p := &PayerProcessor{
		client:       client,
		deposits:     dep,
		payer:        payer,
		bcClient:     bcClient,
		payerWallets: pWal,
		hotWallets:   hWal,
	}
	return p
}

func (p *PayerProcessor) Start() {
	go p.balanceMonitor()
	if !onlyMonitoring {
		go p.startPayments()
		go p.startWithdrawals()
	}
}

func (p *PayerProcessor) balanceMonitor() {
	startTotal := make(map[string]int64)
	for {
		tot := make(map[string]int64)
		for cur, wal := range p.payerWallets {
			var (
				bal, hotBalance *big.Int
				err             error
			)
			if cur == core.TonSymbol {
				bal, _, err = p.bcClient.GetAccountCurrentState(context.Background(), wal.Address)
				if err != nil {
					fmt.Printf("can not get %v payer wallet balance: %v\n", cur, err)
					continue
				}
				hotBalance, _, err = p.bcClient.GetAccountCurrentState(context.Background(), p.hotWallets[cur].Address)
			} else {
				bal, err = p.bcClient.GetLastJettonBalance(context.Background(), wal.Address)
				if err != nil {
					fmt.Printf("can not get %v payer wallet balance: %v\n", cur, err)
					continue
				}
				hotBalance, err = p.bcClient.GetLastJettonBalance(context.Background(), p.hotWallets[cur].Address)
			}
			if err != nil {
				fmt.Printf("can not get %v hot wallet balance: %v\n", cur, err)
				continue
			}

			p.mutex.Lock()
			p.payerWallets[cur] = Wallet{Address: p.payerWallets[cur].Address, Balance: bal}
			p.hotWallets[cur] = Wallet{Address: p.hotWallets[cur].Address, Balance: hotBalance}
			p.mutex.Unlock()
			payerWalletBalance.WithLabelValues(cur).Set(float64(bal.Int64()))
			hotWalletBalance.WithLabelValues(cur).Set(float64(hotBalance.Int64()))

			tot[cur] = bal.Int64() + hotBalance.Int64()

		}
		for _, d := range p.deposits {
			var (
				bal *big.Int
				err error
			)
			if d.Currency == core.TonSymbol {
				bal, _, err = p.bcClient.GetAccountCurrentState(context.Background(), d.Address)
				if err != nil {
					fmt.Printf("can not get %v deposit wallet balance: %v\n", d.Currency, err)
					continue
				}
			} else {
				bal, err = p.bcClient.GetLastJettonBalance(context.Background(), d.JettonWallet)
				if err != nil {
					fmt.Printf("can not get %v payer wallet balance: %v\n", d.Currency, err)
					continue
				}
			}
			tot[d.Currency] = tot[d.Currency] + bal.Int64()
			depositWalletBalance.WithLabelValues(d.Currency, d.Address.String()).Set(float64(bal.Int64()))
		}
		for c, t := range tot {
			totalBalance.WithLabelValues(c).Set(float64(t))
			if _, ok := startTotal[c]; !ok {
				startTotal[c] = t
			}
			totalLosses.WithLabelValues(c).Set(float64(t - startTotal[c]))
		}
		time.Sleep(time.Millisecond * 500)
	}
}

func (p *PayerProcessor) startPayments() {
	for {
		var (
			msgs []*wallet.Message
			skip bool
		)
		time.Sleep(time.Second * 60)
		p.mutex.Lock()
		for cur, wal := range p.payerWallets {
			if wal.Balance.Int64() < int64(payAmount*depositsQty) {
				skip = true
				fmt.Printf("%v not enouth %v balance to pay\n", wal.Balance.Int64(), cur)
			}
		}
		p.mutex.Unlock()

		if skip {
			continue
		}

		for _, dep := range p.deposits {
			t := core.ExternalWithdrawalTask{
				QueryID:     rand.Int63(),
				Currency:    dep.Currency,
				Amount:      core.NewCoins(big.NewInt(payAmount)),
				Destination: core.AddressMustFromTonutilsAddress(dep.Address),
				Bounceable:  false,
				Comment:     "test payment",
			}
			if dep.Currency == core.TonSymbol {
				msgs = append(msgs, core.BuildTonWithdrawalMessage(t))
			} else {
				p.mutex.Lock()
				msgs = append(msgs, core.BuildJettonWithdrawalMessage(t, p.payer, p.payerWallets[dep.Currency].Address))
				p.mutex.Unlock()
			}
		}
		hash, err := p.payer.SendManyWaitTxHash(context.Background(), msgs)
		if err != nil {
			fmt.Printf("send payment message error: %v\n", err)
			continue
		}
		fmt.Printf("sending %v payments with tx hash %x\n", len(msgs), hash)
	}
}

type status struct {
	Pending bool
	ID      int64
}

func (p *PayerProcessor) startWithdrawals() {
	statuses := make(map[string]status)
	for {
		p.mutex.Lock()
		for cur := range p.payerWallets {
			if stat, ok := statuses[cur]; ok {
				st, err := p.client.GetWithdrawalStatus(stat.ID)
				if err != nil {
					fmt.Printf("can not send TON withdrawal: %v\n", err)
					continue
				}
				if st.Status != core.ProcessedStatus {
					continue
				}
			}
			if p.hotWallets[cur].Balance.Int64() > int64(withdrawCutoff) {
				resp, err := p.client.SendWithdrawal(cur, p.payerWallets[core.TonSymbol].Address.String(), p.hotWallets[cur].Balance.Int64()-withdrawCutoff)
				if err != nil {
					fmt.Printf("can not send TON withdrawal: %v\n", err)
				}
				fmt.Printf("Make new %v withdrawal\n", cur)
				statuses[cur] = status{Pending: true, ID: resp.ID}
			}
		}
		p.mutex.Unlock()
		time.Sleep(time.Second * 15)
	}
}
