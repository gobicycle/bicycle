package config

import (
	"github.com/caarlos0/env/v6"
	"github.com/shopspring/decimal"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"log"
	"math/big"
	"strings"
	"time"
)

var (
	JettonTransferTonAmount = tlb.FromNanoTONU(100_000_000)
	JettonForwardAmount     = tlb.FromNanoTONU(20_000_000) // must be < JettonTransferTonAmount

	ExternalMessageLifetime = 50 * time.Second

	ExternalWithdrawalPeriod  = 30 * time.Second
	InternalWithdrawalPeriod  = 5 * time.Second
	ExpirationProcessorPeriod = 5 * time.Second

	AllowableBlockchainLagging     = 15 * time.Second
	AllowableServiceToNodeTimeDiff = 2 * time.Second
)

// JettonProxyContractCode source code at https://github.com/gobicycle/ton-proxy-contract
const JettonProxyContractCode = "B5EE9C72410102010037000114FF00F4A413F4BCF2C80B010050D33331D0D3030171B0915BE0FA4030ED44D0FA4030C705F2E1939320D74A97D4018100A0FB00E8301E8A9040"

var Config = struct {
	LiteServer          string `env:"LITESERVER,required"`
	LiteServerKey       string `env:"LITESERVER_KEY,required"`
	Seed                string `env:"SEED,required"`
	DatabaseURI         string `env:"DB_URI,required"`
	APIHost             string `env:"API_HOST" envDefault:"localhost:8081"`
	APIToken            string `env:"API_TOKEN,required"`
	Testnet             bool   `env:"IS_TESTNET" envDefault:"true"`
	ColdWalletString    string `env:"COLD_WALLET"`
	JettonString        string `env:"JETTONS"`
	TonString           string `env:"TON_CUTOFFS,required"`
	DepositSideBalances bool   `env:"DEPOSIT_SIDE_BALANCE" envDefault:"false"`
	QueueURI            string `env:"QUEUE_URI"`
	QueueName           string `env:"QUEUE_NAME"`
	QueueEnabled        bool   `env:"QUEUE_ENABLED" envDefault:"false"`
	Jettons             map[string]Jetton
	Ton                 Cutoffs
	ColdWallet          *address.Address
}{}

type Jetton struct {
	Master             *address.Address
	WithdrawalCutoff   *big.Int
	HotWalletMaxCutoff *big.Int
}

type Cutoffs struct {
	HotWalletMin *big.Int
	HotWalletMax *big.Int
	Withdrawal   *big.Int
}

func GetConfig() {
	err := env.Parse(&Config)
	if err != nil {
		log.Fatalf("Can not load config")
	}
	Config.Jettons = parseJettonString(Config.JettonString)
	Config.Ton = parseTonString(Config.TonString)

	if Config.ColdWalletString != "" {
		coldAddr, err := address.ParseAddr(Config.ColdWalletString)
		if err != nil {
			log.Fatalf("Can not parse cold wallet address: %v", err)
		}
		if coldAddr.Type() != address.StdAddress {
			log.Fatalf("Only std cold wallet address supported")
		}
		if coldAddr.IsTestnetOnly() && !Config.Testnet {
			log.Fatalf("Can not use testnet cold wallet address for mainnet")
		}
		Config.ColdWallet = coldAddr
	}
}

func parseJettonString(s string) map[string]Jetton {
	res := make(map[string]Jetton)
	jettons := strings.Split(s, ",")
	for _, j := range jettons {
		data := strings.Split(j, ":")
		if len(data) != 4 {
			log.Fatalf("invalid jetton data")
		}
		cur := data[0]
		addr, err := address.ParseAddr(data[1])
		if err != nil {
			log.Fatalf("invalid jetton address: %v", err)
		}
		maxCutoff, err := decimal.NewFromString(data[2])
		if err != nil {
			log.Fatalf("invalid %v jetton max cutoff: %v", data[0], err)
		}
		withdrawalCutoff, err := decimal.NewFromString(data[3])
		if err != nil {
			log.Fatalf("invalid %v jetton withdrawal cutoff: %v", data[0], err)
		}
		res[cur] = Jetton{
			Master:             addr,
			WithdrawalCutoff:   withdrawalCutoff.BigInt(),
			HotWalletMaxCutoff: maxCutoff.BigInt(),
		}
	}
	return res
}

func parseTonString(s string) Cutoffs {
	data := strings.Split(s, ":")
	if len(data) != 3 {
		log.Fatalf("invalid jetton data")
	}
	hotWalletMin, err := decimal.NewFromString(data[0])
	if err != nil {
		log.Fatalf("invalid TON hot wallet min cutoff: %v", err)
	}
	hotWalletMax, err := decimal.NewFromString(data[1])
	if err != nil {
		log.Fatalf("invalid TON hot wallet max cutoff: %v", err)
	}
	withdrawal, err := decimal.NewFromString(data[2])
	if err != nil {
		log.Fatalf("invalid TON withdrawal cutoff: %v", err)
	}
	if hotWalletMin.Cmp(hotWalletMax) == 1 {
		log.Fatalf("TON hot wallet max cutoff must be greater than TON hot wallet min cutoff")
	}
	return Cutoffs{
		HotWalletMin: hotWalletMin.BigInt(),
		HotWalletMax: hotWalletMax.BigInt(),
		Withdrawal:   withdrawal.BigInt(),
	}
}
