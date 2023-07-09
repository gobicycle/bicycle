package api

import (
	"fmt"
	"github.com/go-faster/errors"
	"github.com/gobicycle/bicycle/internal/oas"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/contract/dns"
	"go.uber.org/zap"
	"sync"
)

// Compile-time check for Handler.
var _ oas.Handler = (*Handler)(nil)

type Handler struct {
	oas.UnimplementedHandler // automatically implement all methods
	storage                  storage
	executor                 blockchainExecutor
	dns                      *dns.DNS
	mutex                    sync.Mutex
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage  storage
	executor blockchainExecutor
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}

func WithExecutor(e blockchainExecutor) Option {
	return func(o *Options) {
		o.executor = e
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
	if options.executor == nil {
		return nil, fmt.Errorf("executor is not configured")
	}
	dnsClient := dns.NewDNS(tongo.MustParseAccountID("-1:e56754f83426f69b09267bd876ac97c44821345b7e266bd956a7bfbfb98df35c"), options.executor) //todo: move to chain config

	return &Handler{
		storage:  options.storage,
		executor: options.executor,
		dns:      dnsClient,
	}, nil
}
