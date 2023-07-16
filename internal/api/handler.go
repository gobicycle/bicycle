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
	blockchain               blockchain
	dns                      *dns.DNS
	mutex                    sync.Mutex
	shard                    tongo.ShardID
}

// Options configures behavior of a Handler instance.
type Options struct {
	storage    storage
	blockchain blockchain
	shard      tongo.ShardID
}

type Option func(o *Options)

func WithStorage(s storage) Option {
	return func(o *Options) {
		o.storage = s
	}
}

func WithBlockchain(e blockchain) Option {
	return func(o *Options) {
		o.blockchain = e
	}
}

func WithShard(s tongo.ShardID) Option {
	return func(o *Options) {
		o.shard = s
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
	dnsClient := dns.NewDNS(tongo.MustParseAccountID("-1:e56754f83426f69b09267bd876ac97c44821345b7e266bd956a7bfbfb98df35c"), options.blockchain) //todo: move to chain config

	return &Handler{
		storage:    options.storage,
		blockchain: options.blockchain,
		dns:        dnsClient,
	}, nil
}
