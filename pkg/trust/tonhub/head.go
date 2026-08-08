package tonhub

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gobicycle/bicycle/pkg/trust"
	"github.com/xssnick/tonutils-go/ton"
)

type Tonhub struct {
}

func (Tonhub) CheckBlock(id ton.BlockIDExt) error {
	r, err := http.Get(fmt.Sprintf("https://mainnet-v4.tonhubapi.com/block/%v", id.SeqNo))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	var body struct {
		Block struct {
			Shards []struct {
				Workchain int64  `json:"workchain"`
				RootHash  string `json:"rootHash"`
				FileHash  string `json:"fileHash"`
			}
		}
	}
	err = json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		return err
	}
	for _, block := range body.Block.Shards {
		if block.Workchain == -1 {
			rootHash, err := base64.StdEncoding.DecodeString(block.RootHash)
			if err != nil {
				return err
			}
			fileHash, err := base64.StdEncoding.DecodeString(block.FileHash)
			if err != nil {
				return err
			}
			if !bytes.Equal(rootHash, id.RootHash[:]) || !bytes.Equal(fileHash, id.FileHash[:]) {
				return trust.MismatchHead
			}
			return nil
		}
	}
	return errors.New("block not found in tonhub api")
}
