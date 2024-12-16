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

const MaxJettonForwardTonAmount = 20_000_000

var (
	JettonTransferTonAmount     = tlb.FromNanoTONU(100_000_000)
	JettonForwardAmount         = tlb.FromNanoTONU(MaxJettonForwardTonAmount) // must be < JettonTransferTonAmount
	JettonInternalForwardAmount = tlb.FromNanoTONU(1)

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

const MaxCommentLength = 1000 // qty in chars

var Config = struct {
	LiteServer               string `env:"LITESERVER,required"`
	LiteServerKey            string `env:"LITESERVER_KEY,required"`
	Seed                     string `env:"SEED,required"`
	DatabaseURI              string `env:"DB_URI,required"`
	APIPort                  int    `env:"API_PORT,required"`
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
	ForwardTonAmount         int    `env:"FORWARD_TON_AMOUNT" envDefault:"1"`
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
		log.Fatalf("Can not load config: %v", err)
	}
	Config.Jettons = parseJettonString(Config.JettonString)
	Config.Ton = parseTonString(Config.TonString)

	if Config.ForwardTonAmount < 0 || Config.ForwardTonAmount > MaxJettonForwardTonAmount {
		log.Fatalf("Forward TON amount for jetton transfer must be positive and less than %d", MaxJettonForwardTonAmount)
	} else {
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
