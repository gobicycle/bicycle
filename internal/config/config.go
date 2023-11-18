package config

import (
	"fmt"
	"github.com/caarlos0/env/v6"
	"github.com/shopspring/decimal"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/config"
	"github.com/tonkeeper/tongo/tlb"
	"math/big"
	"reflect"
	"strings"
	"time"
)

var (
	JettonTransferTonAmount = tlb.Coins(100_000_000)
	JettonForwardAmount     = tlb.Coins(20_000_000) // must be < JettonTransferTonAmount

	ExternalMessageLifetime = 50 * time.Second

	ExternalWithdrawalPeriod  = 80 * time.Second // must be ExternalWithdrawalPeriod > ExternalMessageLifetime and some time for balance update
	InternalWithdrawalPeriod  = 30 * time.Second
	ExpirationProcessorPeriod = 5 * time.Second

	AllowableBlockchainLagging     = 15 * time.Second
	AllowableServiceToNodeTimeDiff = 2 * time.Second
)

type Config struct {
	API struct {
		Host  string `env:"API_HOST" envDefault:"0.0.0.0:8081"`
		Token string `env:"API_TOKEN,required"`
	}
	DB struct {
		URI string `env:"DB_URI,required"`
	}
	Webhook struct {
		Endpoint string `env:"WEBHOOK_ENDPOINT"`
		Token    string `env:"WEBHOOK_TOKEN"`
	}
	Blockchain struct {
		ShardLiteserver   config.LiteServer   `env:"SHARD_LITESERVER,required"`
		ShardDepth        int                 `env:"SHARD_DEPTH" envDefault:"0"`
		GlobalLiteservers []config.LiteServer `env:"GLOBAL_LITESERVERS"`
		Testnet           bool                `env:"IS_TESTNET" envDefault:"true"`
	}
	Queue struct {
		URI  string `env:"QUEUE_URI"` // TODO: enabled if present
		Name string `env:"QUEUE_NAME"`
	}
	Processor struct {
		Seed                     string            `env:"SEED,required"`
		ColdWallet               tongo.AccountID   `env:"COLD_WALLET"`
		Jettons                  map[string]Jetton `env:"JETTONS"`
		Ton                      Cutoffs           `env:"TON_CUTOFFS,required"`
		IsDepositSideCalculation bool              `env:"DEPOSIT_SIDE_CALCULATION" envDefault:"true"`
	}
	App struct {
		LogLevel string `env:"LOG_LEVEL" envDefault:"INFO"`
	}
}

type Jetton struct {
	Master             tongo.AccountID
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

func Load() Config {
	var cfg Config

	if err := env.ParseWithFuncs(&cfg, map[reflect.Type]env.ParserFunc{
		// ShardLiteserver
		reflect.TypeOf(config.LiteServer{}): func(v string) (interface{}, error) {
			servers, err := config.ParseLiteServersEnvVar(v)
			if err != nil {
				return nil, err
			}
			if len(servers) != 1 {
				return nil, fmt.Errorf("must be only one shard litesrever")
			}
			return servers, nil
		},
		// GlobalLiteservers
		reflect.TypeOf([]config.LiteServer{}): func(v string) (interface{}, error) {
			servers, err := config.ParseLiteServersEnvVar(v)
			if err != nil {
				return nil, err
			}
			return servers, nil
		},
		// ColdWallet
		reflect.TypeOf(tongo.AccountID{}): func(v string) (interface{}, error) {
			id, err := tongo.ParseAccountID(v)
			if err != nil {
				return nil, err
			}
			return id, nil
		},
		// Ton
		reflect.TypeOf(Cutoffs{}): func(v string) (interface{}, error) {
			ton, err := parseTonString(v)
			if err != nil {
				return nil, err
			}
			return *ton, nil
		},
		// Jettons
		reflect.TypeOf(map[string]Jetton{}): func(v string) (interface{}, error) {
			return parseJettonString(v)
		},
	}); err != nil {
		panic("Config parsing failed: " + err.Error() + "\n")
	}

	// TODO: clarify checks
	if cfg.Blockchain.ShardDepth > 0 && len(cfg.Blockchain.GlobalLiteservers) == 0 {
		panic("must be at least one global liteserver for non zero shard depth")
	}

	// TODO: check cold wallet address for testnet and bounce
	// TODO: load network config from node

	return cfg
}

func parseJettonString(s string) (map[string]Jetton, error) {
	res := make(map[string]Jetton)
	if s == "" {
		return res, nil
	}
	jettons := strings.Split(s, ",")
	for _, j := range jettons {
		data := strings.Split(j, ":")
		if len(data) != 5 {
			return nil, fmt.Errorf("invalid jetton data")
		}
		cur := data[0]
		addr, err := tongo.ParseAccountID(data[1])
		if err != nil {
			return nil, fmt.Errorf("invalid jetton address: %w", err)
		}
		maxCutoff, err := decimal.NewFromString(data[2])
		if err != nil {
			return nil, fmt.Errorf("invalid %s jetton max cutoff: %w", data[0], err)
		}
		withdrawalCutoff, err := decimal.NewFromString(data[3])
		if err != nil {
			return nil, fmt.Errorf("invalid %s jetton withdrawal cutoff: %w", data[0], err)
		}
		residual, err := decimal.NewFromString(data[4])
		if err != nil {
			return nil, fmt.Errorf("invalid %s jetton hot wallet withdrawal residual: %w", data[0], err)
		}
		res[cur] = Jetton{
			Master:             addr,
			WithdrawalCutoff:   withdrawalCutoff.BigInt(),
			HotWalletMaxCutoff: maxCutoff.BigInt(),
			HotWalletResidual:  residual.BigInt(),
		}
	}
	return res, nil
}

func parseTonString(s string) (*Cutoffs, error) {
	data := strings.Split(s, ":")
	if len(data) != 4 {
		return nil, fmt.Errorf("invalid TON data")
	}
	hotWalletMin, err := decimal.NewFromString(data[0])
	if err != nil {
		return nil, fmt.Errorf("invalid TON hot wallet min cutoff: %w", err)
	}
	hotWalletMax, err := decimal.NewFromString(data[1])
	if err != nil {
		return nil, fmt.Errorf("invalid TON hot wallet max cutoff: %w", err)
	}
	withdrawal, err := decimal.NewFromString(data[2])
	if err != nil {
		return nil, fmt.Errorf("invalid TON withdrawal cutoff: %w", err)
	}
	if hotWalletMin.Cmp(hotWalletMax) == 1 {
		return nil, fmt.Errorf("TON hot wallet max cutoff must be greater than TON hot wallet min cutoff")
	}
	residual, err := decimal.NewFromString(data[3])
	if err != nil {
		return nil, fmt.Errorf("invalid TON hot wallet withdrawal residual: %w", err)
	}
	if residual.Cmp(hotWalletMax) == 1 {
		return nil, fmt.Errorf("TON hot wallet max cutoff must be greater than TON hot wallet residual")
	}

	return &Cutoffs{
		HotWalletMin:      hotWalletMin.BigInt(),
		HotWalletMax:      hotWalletMax.BigInt(),
		Withdrawal:        withdrawal.BigInt(),
		HotWalletResidual: residual.BigInt(),
	}, nil
}
