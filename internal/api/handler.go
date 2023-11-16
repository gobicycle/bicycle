package api

import (
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/oas"
	"go.uber.org/zap"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
	generator                depositGenerator
	incomeCountingSide       core.IncomeSide
	isTestnet                bool // TODO: make special type instead of bool
	jettons                  map[string]struct{}
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage            storage
	generator          depositGenerator
	incomeCountingSide core.IncomeSide
	isTestnet          *bool
	jettons            map[string]struct{}
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}

func WithDepositGenerator(g depositGenerator) Option {
	return func(o *Options) {
		o.generator = g
	}
}

func WithIncomeCountingSide(incomeCountingSide core.IncomeSide) Option {
	return func(o *Options) {
		o.incomeCountingSide = incomeCountingSide
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
	if options.generator == nil {
		return nil, fmt.Errorf("deposit generator is not configured")
	}
	if options.isTestnet == nil {
		return nil, fmt.Errorf("testnet flag is not configured")
	}
	// TODO: check other options

	return &Handler{
		storage:            options.storage,
		incomeCountingSide: options.incomeCountingSide,
		isTestnet:          *options.isTestnet,
		generator:          options.generator,
	}, nil
}
