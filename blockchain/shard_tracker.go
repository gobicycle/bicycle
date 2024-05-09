package blockchain

import (
	"context"
	"github.com/gobicycle/bicycle/core"
	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/tlb"
	"github.com/xssnick/tonutils-go/ton"
	"math/bits"
	"strings"
	"time"
)

const ErrBlockNotApplied = "block is not applied"
const ErrBlockNotInDB = "code 651"

type ShardTracker struct {
	connection            *Connection
	shard                 byte
	lastKnownShardBlock   *ton.BlockIDExt
	lastMasterBlock       *ton.BlockIDExt
	buffer                []core.ShardBlockHeader
	gracefulShutdown      bool
	infoCounter, infoStep int
	infoLastTime          time.Time
}

// NewShardTracker creates new tracker to get blocks with specific shard attribute
func NewShardTracker(shard byte, startBlock *ton.BlockIDExt, connection *Connection) *ShardTracker {
	t := &ShardTracker{
		connection:          connection,
		shard:               shard,
		lastKnownShardBlock: startBlock,
		buffer:              make([]core.ShardBlockHeader, 0),
		infoCounter:         0,
		infoStep:            1000,
		infoLastTime:        time.Now(),
	}
	return t
}

// NextBlock returns next block header and graceful shutdown flag.
// (ShardBlockHeader, false) for normal operation and (empty block header, true) for graceful shutdown.
func (s *ShardTracker) NextBlock() (core.ShardBlockHeader, bool, error) {
	if s.gracefulShutdown {
		return core.ShardBlockHeader{}, true, nil
	}
	h := s.getNext()
	if h != nil {
		return *h, false, nil
	}
	// the interval between blocks can be up to 40 seconds
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60)
	defer cancel()
	masterBlockID, err := s.getNextMasterBlockID(ctx)
	if err != nil {
		return core.ShardBlockHeader{}, false, err
	}
	exit, err := s.loadShardBlocksBatch(masterBlockID)
	if err != nil {
		return core.ShardBlockHeader{}, false, err
	}
	if exit {
		log.Printf("Shard tracker sync stopped")
		return core.ShardBlockHeader{}, true, nil
	}
	return s.NextBlock()
}

// Stop initiates graceful shutdown
func (s *ShardTracker) Stop() {
	s.gracefulShutdown = true
}

func (s *ShardTracker) getNext() *core.ShardBlockHeader {
	if len(s.buffer) != 0 {
		h := s.buffer[0]
		s.buffer = s.buffer[1:]
		return &h
	}
	return nil
}

func (s *ShardTracker) getNextMasterBlockID(ctx context.Context) (*ton.BlockIDExt, error) {
	for {
		masterBlockID, err := s.connection.client.GetMasterchainInfo(ctx)
		if err != nil {
			// exit by context timeout
			return nil, err
		}
		if s.lastMasterBlock == nil {
			s.lastMasterBlock = masterBlockID
			return masterBlockID, nil
		}
		if masterBlockID.SeqNo == s.lastMasterBlock.SeqNo {
			time.Sleep(time.Second)
			continue
		}
		s.lastMasterBlock = masterBlockID
		return masterBlockID, nil
	}
}

func (s *ShardTracker) loadShardBlocksBatch(masterBlockID *ton.BlockIDExt) (bool, error) {
	var (
		shards []*ton.BlockIDExt
		err    error
	)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	for {
		shards, err = s.connection.client.GetBlockShardsInfo(ctx, masterBlockID)
		if err != nil && isNotReadyError(err) { // TODO: clarify error type
			time.Sleep(time.Second)
			continue
		} else if err != nil {
			return false, err
			// exit by context timeout
		}
		break
	}
	s.infoCounter = 0
	batch, exit, err := s.getShardBlocksRecursively(filterByShard(shards, s.shard), nil)
	if err != nil {
		return false, err
	}
	if exit {
		return true, nil
	}
	if len(batch) != 0 {
		s.lastKnownShardBlock = batch[0].BlockIDExt
		for i := len(batch) - 1; i >= 0; i-- {
			s.buffer = append(s.buffer, batch[i])
		}
	}
	return false, nil
}

func (s *ShardTracker) getShardBlocksRecursively(i *ton.BlockIDExt, batch []core.ShardBlockHeader) ([]core.ShardBlockHeader, bool, error) {
	if s.gracefulShutdown {
		return nil, true, nil
	}
	if s.lastKnownShardBlock == nil {
		s.lastKnownShardBlock = i
	}
	isKnown := (s.lastKnownShardBlock.Shard == i.Shard) && (s.lastKnownShardBlock.SeqNo == i.SeqNo)
	if isKnown {
		return batch, false, nil
	}

	// compare seqno with filtered shard block
	// handle the case when a node may reference an old block
	if s.lastKnownShardBlock.SeqNo > i.SeqNo {
		return []core.ShardBlockHeader{}, false, nil
	}

	seqnoDiff := int(i.SeqNo - s.lastKnownShardBlock.SeqNo)
	if seqnoDiff > s.infoStep {
		if s.infoCounter%s.infoStep == 0 {
			estimatedTime := time.Duration(seqnoDiff/s.infoStep) * time.Since(s.infoLastTime)
			s.infoLastTime = time.Now()
			if s.infoCounter == 0 {
				log.Printf("Shard tracker syncing... Seqno diff: %v Estimated time: unknown\n", seqnoDiff)
			} else {
				log.Printf("Shard tracker syncing... Seqno diff: %v Estimated time: %v\n", seqnoDiff, estimatedTime)
			}
		}
		s.infoCounter++
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*20)
	defer cancel()
	h, err := s.connection.getShardBlocksHeader(ctx, i, s.shard)
	if err != nil {
		return nil, false, err
	}
	batch = append(batch, h)
	return s.getShardBlocksRecursively(h.Parent, batch)
}

func isInShard(blockShardPrefix uint64, shard byte) bool {
	if blockShardPrefix == 0 {
		log.Fatalf("invalid shard_prefix")
	}
	prefixLen := 64 - 1 - bits.TrailingZeros64(blockShardPrefix) // without one insignificant bit
	if prefixLen > 8 {
		log.Fatalf("more than 256 shards is not supported")
	}
	res := (uint64(shard) << (64 - 8)) ^ blockShardPrefix

	return bits.LeadingZeros64(res) >= prefixLen
}

func filterByShard(headers []*ton.BlockIDExt, shard byte) *ton.BlockIDExt {
	for _, h := range headers {
		if isInShard(uint64(h.Shard), shard) {
			return h
		}
	}
	log.Fatalf("must be at least one suitable shard block")
	return nil
}

func convertBlockToShardHeader(block *tlb.Block, info *ton.BlockIDExt, shard byte) (core.ShardBlockHeader, error) {
	parents, err := block.BlockInfo.GetParentBlocks()
	if err != nil {
		return core.ShardBlockHeader{}, err
	}
	parent := filterByShard(parents, shard)
	return core.ShardBlockHeader{
		NotMaster:  block.BlockInfo.NotMaster,
		GenUtime:   block.BlockInfo.GenUtime,
		StartLt:    block.BlockInfo.StartLt,
		EndLt:      block.BlockInfo.EndLt,
		Parent:     parent,
		BlockIDExt: info,
	}, nil
}

// get shard block header for specific shard attribute with one parent
func (c *Connection) getShardBlocksHeader(ctx context.Context, shardBlockID *ton.BlockIDExt, shard byte) (core.ShardBlockHeader, error) {
	var (
		err   error
		block *tlb.Block
	)
	for {
		block, err = c.client.GetBlockData(ctx, shardBlockID)
		if err != nil && isNotReadyError(err) {
			continue
		} else if err != nil {
			return core.ShardBlockHeader{}, err
			// exit by context timeout
		}
		break
	}
	return convertBlockToShardHeader(block, shardBlockID, shard)
}

func isNotReadyError(err error) bool {
	return strings.Contains(err.Error(), ErrBlockNotApplied) || strings.Contains(err.Error(), ErrBlockNotInDB)
}
