package toncenter

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

type Toncenter struct {
}

func (Toncenter) CheckBlock(id ton.BlockIDExt) error {
	r, err := http.Get(fmt.Sprintf("https://toncenter.com/api/v3/blocks?workchain=%v&shard=%x&seqno=%v&limit=10&offset=0&sort=desc", id.Workchain, uint64(id.Shard), id.SeqNo))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("tonapi bad status: %s", r.Status)
	}
	var resp struct {
		Blocks []struct {
			RootHash string `json:"root_hash"`
			FileHash string `json:"file_hash"`
		}
	}

	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return err
	}
	if len(resp.Blocks) == 0 {
		return errors.New("block not found in toncenter response")
	}
	rootHash, err := base64.StdEncoding.DecodeString(resp.Blocks[0].RootHash)
	if err != nil {
		return err
	}
	fileHash, err := base64.StdEncoding.DecodeString(resp.Blocks[0].FileHash)
	if err != nil {
		return err
	}
	if !bytes.Equal(rootHash, id.RootHash[:]) || !bytes.Equal(fileHash, id.FileHash[:]) {
		return trust.MismatchHead
	}
	return nil
}
