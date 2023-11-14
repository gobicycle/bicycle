package api

import (
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"go.uber.org/zap"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
	blockchain               blockchain
	dns                      *dns.DNS
	generator                *core.DepositGenerator // TODO: maybe use interface
	isDepositSideCalculation bool                   // TODO: make special type instead of bool
	isTestnet                bool
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage                  storage
	blockchain               blockchain
	generator                *core.DepositGenerator
	isDepositSideCalculation bool
	isTestnet                *bool
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}

func WithBlockchain(b blockchain) Option {
	return func(o *Options) {
		o.blockchain = b
	}
}

func WithDepositGenerator(g *core.DepositGenerator) Option {
	return func(o *Options) {
		o.generator = g
	}
}

func WithDepositSide(isDepositSideCalculation bool) Option {
	return func(o *Options) {
		o.isDepositSideCalculation = isDepositSideCalculation
	}
}

func WithTestnetFlag(isTestnet bool) Option {
	return func(o *Options) {
		o.isTestnet = &isTestnet
	}
}

func NewHandler(logger *zap.Logger, opts ...Option) (*Handler, error) {
	options := &Options{}
	for _, o := range opts {
		o(options)
	}
	if options.storage == nil {
		return nil, errors.New("storage is not configured")
	}
	if options.blockchain == nil {
		return nil, fmt.Errorf("blockchain is not configured")
	}
	if options.generator == nil {
		return nil, fmt.Errorf("deposit generator is not configured")
	}
	if options.isTestnet == nil {
		return nil, fmt.Errorf("testnet flag is not configured")
	}
	// TODO: check other options
	dnsClient := dns.NewDNS(tongo.MustParseAccountID("-1:e56754f83426f69b09267bd876ac97c44821345b7e266bd956a7bfbfb98df35c"), options.blockchain) //todo: move to chain config

	return &Handler{
		storage:                  options.storage,
		blockchain:               options.blockchain,
		dns:                      dnsClient,
		isDepositSideCalculation: options.isDepositSideCalculation,
		isTestnet:                *options.isTestnet,
		generator:                options.generator,
	}, nil
}
