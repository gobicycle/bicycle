package config

import (
	"github.com/caarlos0/env/v6"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo/boc"
	"github.com/xssnick/tonutils-go/address"
	"github.com/xssnick/tonutils-go/tlb"
	"log"
	"math/big"
	"strings"
	"time"
)

const DefaultJettonForwardTonAmount = 20_000_000

var (
	JettonTransferTonAmount = tlb.FromNanoTONU(100_000_000)
	JettonForwardAmount     = tlb.FromNanoTONU(DefaultJettonForwardTonAmount) // must be < JettonTransferTonAmount

	DefaultHotWalletHysteresis = decimal.NewFromFloat(0.95) // `hot_wallet_residual_balance` = `hot_wallet_max_balance` * `hysteresis`

	ExternalMessageLifetime = 50 * time.Second

	ExternalWithdrawalPeriod  = 80 * time.Second // must be ExternalWithdrawalPeriod > ExternalMessageLifetime and some time for balance update
	InternalWithdrawalPeriod  = 80 * time.Second
	ExpirationProcessorPeriod = 5 * time.Second

	AllowableBlockchainLagging     = 40 * time.Second // TODO: use env var
	AllowableServiceToNodeTimeDiff = 2 * time.Second
)

// JettonProxyContractCode source code at https://github.com/gobicycle/ton-proxy-contract
const JettonProxyContractCode = "B5EE9C72410102010037000114FF00F4A413F4BCF2C80B010050D33331D0D3030171B0915BE0FA4030ED44D0FA4030C705F2E1939320D74A97D4018100A0FB00E8301E8A9040"

const (
	TestnetConfig = "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"
)

const MaxCommentLength = 1000 // qty in chars

var Config = struct {
	LiteServer               string `env:"LITESERVER,required"`
	LiteServerKey            string `env:"LITESERVER_KEY,required"`
	Seed                     string `env:"SEED,required"`
	DatabaseURI              string `env:"DB_URI,required"`
	APIHost                  string `env:"API_HOST" envDefault:"0.0.0.0:8081"`
	APIToken                 string `env:"API_TOKEN,required"`
	Testnet                  bool   `env:"IS_TESTNET" envDefault:"true"`
	ColdWalletString         string `env:"COLD_WALLET"`
	JettonString             string `env:"JETTONS"`
	TonString                string `env:"TON_CUTOFFS,required"`
	IsDepositSideCalculation bool   `env:"DEPOSIT_SIDE_BALANCE" envDefault:"true"` // TODO: rename to DEPOSIT_SIDE_CALCULATION
	QueueURI                 string `env:"QUEUE_URI"`
	QueueName                string `env:"QUEUE_NAME"`
	QueueEnabled             bool   `env:"QUEUE_ENABLED" envDefault:"false"`
	ProofCheckEnabled        bool   `env:"PROOF_CHECK_ENABLED" envDefault:"false"`
	NetworkConfigUrl         string `env:"NETWORK_CONFIG_URL"`
	WebhookEndpoint          string `env:"WEBHOOK_ENDPOINT"`
	WebhookToken             string `env:"WEBHOOK_TOKEN"`
	AllowableLaggingSec      int    `env:"ALLOWABLE_LAG"`
	ForwardTonAmount         int    `env:"FORWARD_TON_AMOUNT"`
	Jettons                  map[string]Jetton
	Ton                      Cutoffs
	ColdWallet               *address.Address
	BlockchainConfig         *boc.Cell
}{}

type Jetton struct {
	Master             *address.Address
	WithdrawalCutoff   *big.Int
	HotWalletMaxCutoff *big.Int
	HotWalletResidual  *big.Int
}

type Cutoffs struct {
	HotWalletMin      *big.Int
	HotWalletMax      *big.Int
	Withdrawal        *big.Int
	HotWalletResidual *big.Int
}

func GetConfig() {
	err := env.Parse(&Config)
	if err != nil {
		log.Fatalf("Can not load config")
	}
	Config.Jettons = parseJettonString(Config.JettonString)
	Config.Ton = parseTonString(Config.TonString)

	if Config.ForwardTonAmount < 0 || Config.ForwardTonAmount > DefaultJettonForwardTonAmount {
		log.Fatalf("Forward TON amount for jetton transfer must be positive and less than %d", DefaultJettonForwardTonAmount)
	} else if Config.ForwardTonAmount > 0 {
		JettonForwardAmount = tlb.FromNanoTONU(uint64(Config.ForwardTonAmount))
	}

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

	if Config.Testnet {
		config, err := boc.DeserializeBocBase64(TestnetConfig)
		if err != nil {
			log.Fatalf("Can not parse testnet blockchain config: %v", err)
		}
		Config.BlockchainConfig = config[0]
	} else {
		config, err := boc.DeserializeBocBase64(MainnetConfig)
		if err != nil {
			log.Fatalf("Can not parse mainnet blockchain config: %v", err)
		}
		Config.BlockchainConfig = config[0]
	}

	if Config.AllowableLaggingSec != 0 {
		AllowableBlockchainLagging = time.Second * time.Duration(Config.AllowableLaggingSec)
	}
}

func parseJettonString(s string) map[string]Jetton {
	res := make(map[string]Jetton)
	if s == "" {
		return res
	}
	jettons := strings.Split(s, ",")
	for _, j := range jettons {
		data := strings.Split(j, ":")
		if len(data) != 4 && len(data) != 5 {
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

		residual := maxCutoff.Mul(DefaultHotWalletHysteresis)
		if len(data) == 5 {
			residual, err = decimal.NewFromString(data[4])
			if err != nil {
				log.Fatalf("invalid hot_wallet_residual_balance parameter: %v", err)
			}
		}

		res[cur] = Jetton{
			Master:             addr,
			WithdrawalCutoff:   withdrawalCutoff.BigInt(),
			HotWalletMaxCutoff: maxCutoff.BigInt(),
			HotWalletResidual:  residual.BigInt(),
		}
	}
	return res
}

func parseTonString(s string) Cutoffs {
	data := strings.Split(s, ":")
	if len(data) != 3 && len(data) != 4 {
		log.Fatalf("invalid TON cuttofs")
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

	residual := hotWalletMax.Mul(DefaultHotWalletHysteresis)
	if len(data) == 4 {
		residual, err = decimal.NewFromString(data[3])
		if err != nil {
			log.Fatalf("invalid hot_wallet_residual_balance parameter: %v", err)
		}
	}

	return Cutoffs{
		HotWalletMin:      hotWalletMin.BigInt(),
		HotWalletMax:      hotWalletMax.BigInt(),
		Withdrawal:        withdrawal.BigInt(),
		HotWalletResidual: residual.BigInt(),
	}
}
