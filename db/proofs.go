package db

import (
	"context"

	"github.com/xssnick/tonutils-go/ton"
)

func (c *Connection) SaveLastMasterchainProvenBlock(ctx context.Context, block ton.BlockIDExt) error {
	_, err := c.client.Exec(ctx, `
		INSERT INTO payments.last_proven_block (
		slug,
		workchain,   
		shard,
		seqno,                                 
		root_hash,
		file_hash                               
		) VALUES ('head', $1, $2, $3, $4, $5)  
		ON CONFLICT (slug) DO UPDATE SET workchain = $1, shard = $2, seqno = $3, root_hash = $4, file_hash = $5
	`, block.Workchain,
		block.Shard,
		block.SeqNo,
		block.RootHash,
		block.FileHash,
	)
	return err
}

func (c *Connection) GetLastMasterchainProvenBlock(ctx context.Context) (*ton.BlockIDExt, error) {
	var block ton.BlockIDExt
	err := c.client.
		QueryRow(ctx, `SELECT workchain, shard, seqno, root_hash, file_hash FROM payments.last_proven_block WHERE slug = 'head'`).
		Scan(&block.Workchain, &block.Shard, &block.SeqNo, &block.RootHash, &block.FileHash)
	return &block, err
}
