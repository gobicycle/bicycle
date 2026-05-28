package blockchain

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/xssnick/tonutils-go/ton"
)

type provenBlocksStorage interface {
	SaveLastMasterchainProvenBlock(ctx context.Context, block ton.BlockIDExt) error
	GetLastMasterchainProvenBlock(ctx context.Context) (*ton.BlockIDExt, error)
}

func updateLastBlocks(client ton.APIClientWrapped, storage provenBlocksStorage) {
	for {
		time.Sleep(time.Minute)
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		last, err := client.GetMasterchainInfo(ctx)
		if err == nil {
			err := storage.SaveLastMasterchainProvenBlock(ctx, *last)
			if err != nil {
				log.Errorf("save last masterchain block err: %v. If you are using PROOF_CHECK_ENABLED please make migration 02_add_proofs_table.up.sql ", err)
			}
		}
		cancel()
	}
}
