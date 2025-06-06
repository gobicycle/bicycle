package blockchain

import (
	"context"
	"fmt"

	"github.com/xssnick/tonutils-go/tl"
	"github.com/xssnick/tonutils-go/ton"
	"golang.org/x/time/rate"
)

type limitedLiteClient struct {
	limiter  *rate.Limiter
	original ton.LiteClient
}

func newLimitedClient(lc ton.LiteClient, rateLimit int) *limitedLiteClient {
	return &limitedLiteClient{
		original: lc,
		limiter:  rate.NewLimiter(rate.Limit(rateLimit), 1),
	}
}

func (w *limitedLiteClient) QueryLiteserver(ctx context.Context, payload tl.Serializable, result tl.Serializable) error {
	err := w.limiter.Wait(ctx)
	if err != nil {
		return fmt.Errorf("limiter err: %w", err)
	}
	return w.original.QueryLiteserver(ctx, payload, result)
}

func (w *limitedLiteClient) StickyContext(ctx context.Context) context.Context {
	return w.original.StickyContext(ctx)
}

func (w *limitedLiteClient) StickyNodeID(ctx context.Context) uint32 {
	return w.original.StickyNodeID(ctx)
}

func (w *limitedLiteClient) StickyContextNextNode(ctx context.Context) (context.Context, error) {
	return w.original.StickyContextNextNode(ctx)
}

func (w *limitedLiteClient) StickyContextNextNodeBalanced(ctx context.Context) (context.Context, error) {
	return w.original.StickyContextNextNodeBalanced(ctx)
}
