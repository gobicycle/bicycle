package blockchain

import (
	"context"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"time"
)

type ShardTracker struct {
	connection           *Connection
	shardID              tongo.ShardID
	lastMasterBlock      tongo.BlockIDExt
	lastKnownShardBlocks []tongo.BlockIDExt
}

// Options holds parameters to configure a shard tracker instance.
type Options struct {
	StartMasterBlockSeqno *uint32       // Masterchain block ID for init shard tracker
	ShardID               tongo.ShardID // Mask of shard for tracking in format with flip bit
}

type Option func(o *Options) error

// WithStartBlockSeqno for configure first masterchain block seqno for shard tracker
func WithStartBlockSeqno(startMasterBlockSeqno uint32) Option {
	return func(o *Options) error {
		o.StartMasterBlockSeqno = &startMasterBlockSeqno
		return nil
	}
}

// WithShard to define the shard mask to get shard blocks
func WithShard(shardID tongo.ShardID) Option {
	return func(o *Options) error {
		o.ShardID = shardID
		return nil
	}
}

// NewShardTracker creates new tracker to get blocks with specific shard attribute
func NewShardTracker(connection *Connection, opts ...Option) (*ShardTracker, error) {
	defaultShardID, err := tongo.ParseShardID(core.DefaultShard)
	if err != nil {
		return nil, err
	}
	options := &Options{
		ShardID: defaultShardID,
	}

	for _, o := range opts {
		if err := o(options); err != nil {
			return nil, err
		}
	}

	if options.StartMasterBlockSeqno == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		info, err := connection.client.GetMasterchainInfo(ctx)
		if err != nil {
			return nil, err
		}
		options.StartMasterBlockSeqno = &info.Last.Seqno // TODO: clarify seqno
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	lastMasterBlockIDExt, err := connection.lookupMasterchainBlock(ctx, *options.StartMasterBlockSeqno)
	if err != nil {
		return nil, err
	}

	block, err := connection.client.GetBlock(ctx, *lastMasterBlockIDExt)
	if err != nil {
		return nil, err
	}

	shards := tongo.ShardIDs(&block)

	t := &ShardTracker{
		connection:           connection,
		shardID:              options.ShardID,
		lastMasterBlock:      *lastMasterBlockIDExt,
		lastKnownShardBlocks: shards,
	}
	return t, nil
}

// NextBatch returns last scanned master block and batch of workchain blocks, committed to the next master blocks and
// all intermediate blocks before those committed to the last known master block and filtered by shard parameter.
func (s *ShardTracker) NextBatch() ([]*core.ShardBlock, *tlb.Block, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60) // timeout between master blocks may be up tp 60 sec
	defer cancel()

	masterID, err := s.connection.lookupMasterchainBlock(ctx, s.lastMasterBlock.Seqno+1)
	if err != nil {
		return nil, nil, err
	}

	masterBlock, err := s.connection.client.GetBlock(ctx, *masterID)
	if err != nil {
		return nil, nil, err
	}

	shards := tongo.ShardIDs(&masterBlock)

	var batch []*core.ShardBlock
	filteredShardBlocks := s.filterByShard(shards)

	for _, b := range filteredShardBlocks {
		batch, err = s.getShardBlocksRecursively(b, batch)
		if err != nil {
			return nil, nil, err
		}
	}

	s.lastKnownShardBlocks = shards
	s.lastMasterBlock = *masterID

	return batch, &masterBlock, nil
}

func (s *ShardTracker) getShardBlocksRecursively(blockID tongo.BlockIDExt, batch []*core.ShardBlock) ([]*core.ShardBlock, error) {
	if s.isKnownShardBlock(blockID) {
		return batch, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	block, err := s.connection.client.GetBlock(ctx, blockID)
	if err != nil {
		return nil, err
	}

	h, err := convertBlockToShardHeader(&block, blockID)
	if err != nil {
		return nil, err
	}

	batch = append(batch, h)
	for _, p := range h.Parents {
		batch, err = s.getShardBlocksRecursively(p, batch)
		if err != nil {
			return nil, err
		}
	}
	return batch, nil
}

func (s *ShardTracker) isKnownShardBlock(blockID tongo.BlockIDExt) bool {
	for _, lastBlockID := range s.lastKnownShardBlocks {
		if (lastBlockID.Shard == blockID.Shard) && (lastBlockID.Seqno == blockID.Seqno) {
			return true
		}
	}
	return false
}

func (s *ShardTracker) filterByShard(headers []tongo.BlockIDExt) []tongo.BlockIDExt {
	var res []tongo.BlockIDExt
	for _, h := range headers {
		if s.shardID.MatchBlockID(h.BlockID) {
			res = append(res, h)
		}
	}
	// TODO: check for empty slice?
	return res
}

func convertBlockToShardHeader(block *tlb.Block, id tongo.BlockIDExt) (*core.ShardBlock, error) {
	parents, err := tongo.GetParents(block.Info)
	if err != nil {
		return nil, err
	}

	return &core.ShardBlock{
		//IsMaster:   !block.BlockInfo.NotMaster,
		GenUtime: block.Info.GenUtime,
		//StartLt:    block.BlockInfo.StartLt,
		//EndLt:      block.BlockInfo.EndLt,
		BlockIDExt:   id,
		Parents:      parents,
		Transactions: block.AllTransactions(),
	}, nil
}
