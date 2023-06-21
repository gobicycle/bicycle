package blockchain

import (
	"context"
	"github.com/gobicycle/bicycle/core"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"time"
)

type ShardTracker struct {
	connection           *Connection
	shardID              core.ShardID
	lastMasterBlock      *ton.BlockIDExt
	lastKnownShardBlocks []*ton.BlockIDExt
}

// Options holds parameters to configure a shard tracker instance.
type Options struct {
	StartMasterBlockID *ton.BlockIDExt // Masterchain block ID for init shard tracker
	ShardID            core.ShardID    // Mask of shard for tracking in format with flip bit
}

type Option func(o *Options) error

// WithStartBlock for configure first masterchain block ID for shard tracker
func WithStartBlock(startMasterBlockID *ton.BlockIDExt) Option {
	return func(o *Options) error {
		o.StartMasterBlockID = startMasterBlockID
		return nil
	}
}

// WithShard to define the shard mask to get shard blocks
func WithShard(shardID core.ShardID) Option {
	return func(o *Options) error {
		o.ShardID = shardID
		return nil
	}
}

// NewShardTracker creates new tracker to get blocks with specific shard attribute
func NewShardTracker(connection *Connection, opts ...Option) (*ShardTracker, error) {
	defaultShardID, err := core.ParseShardID(core.DefaultShard)
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

	if options.StartMasterBlockID == nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
		defer cancel()
		info, err := connection.client.GetMasterchainInfo(ctx)
		if err != nil {
			return nil, err
		}
		options.StartMasterBlockID = info.Last
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	shards, err := connection.client.GetBlockShardsInfo(ctx, options.StartMasterBlockID)

	if err != nil {
		return nil, err
	}

	t := &ShardTracker{
		connection:           connection,
		shardID:              options.ShardID,
		lastMasterBlock:      options.StartMasterBlockID,
		lastKnownShardBlocks: shards,
	}
	return t, nil
}

// NextBatch returns last scanned master block and batch of workchain blocks, committed to the next master blocks and
// all intermediate blocks before those committed to the last known master block and filtered by shard parameter.
func (s *ShardTracker) NextBatch() ([]*core.ShardBlockHeader, *core.ShardBlockHeader, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60) // timeout between master blocks may be up tp 60 sec
	defer cancel()

	master, err := s.connection.WaitForBlock(s.lastMasterBlock.SeqNo+1).LookupBlock(
		ctx, s.lastMasterBlock.Workchain, s.lastMasterBlock.Shard, s.lastMasterBlock.SeqNo+1)
	if err != nil {
		return nil, nil, err
	}
	masterBlock, err := s.connection.client.GetBlockData(ctx, master)
	if err != nil {
		return nil, nil, err
	}
	masterHeader, err := convertBlockToShardHeader(masterBlock, master)
	if err != nil {
		return nil, nil, err
	}

	shardBlocks, err := s.connection.client.GetBlockShardsInfo(ctx, master)
	if err != nil {
		return nil, nil, err
	}

	var batch []*core.ShardBlockHeader
	filteredShardBlocks := s.filterByShard(shardBlocks)

	for _, b := range filteredShardBlocks {
		batch, err = s.getShardBlocksRecursively(b, batch)
		if err != nil {
			return nil, nil, err
		}
	}

	s.lastKnownShardBlocks = shardBlocks
	s.lastMasterBlock = master

	return batch, masterHeader, nil
}

func (s *ShardTracker) getShardBlocksRecursively(blockID *ton.BlockIDExt, batch []*core.ShardBlockHeader) ([]*core.ShardBlockHeader, error) {
	if s.isKnownShardBlock(blockID) {
		return batch, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()

	block, err := s.connection.client.GetBlockData(ctx, blockID)
	if err != nil {
		return nil, err
	}
	h, err := convertBlockToShardHeader(block, blockID)
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

func (s *ShardTracker) isKnownShardBlock(blockID *ton.BlockIDExt) bool {
	for _, lastBlockID := range s.lastKnownShardBlocks {
		if (lastBlockID.Shard == blockID.Shard) && (lastBlockID.SeqNo == blockID.SeqNo) {
			return true
		}
	}
	return false
}

func (s *ShardTracker) filterByShard(headers []*ton.BlockIDExt) []*ton.BlockIDExt {
	var res []*ton.BlockIDExt
	for _, h := range headers {
		if s.shardID.MatchBlockID(h) {
			res = append(res, h)
		}
	}
	// TODO: check for empty slice?
	return res
}

func convertBlockToShardHeader(block *tlb.Block, info *ton.BlockIDExt) (*core.ShardBlockHeader, error) {
	parents, err := block.BlockInfo.GetParentBlocks()
	if err != nil {
		return nil, err
	}
	return &core.ShardBlockHeader{
		IsMaster:   !block.BlockInfo.NotMaster,
		GenUtime:   block.BlockInfo.GenUtime,
		StartLt:    block.BlockInfo.StartLt,
		EndLt:      block.BlockInfo.EndLt,
		Parents:    parents,
		BlockIDExt: info,
	}, nil
}
