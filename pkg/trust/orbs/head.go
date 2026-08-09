package orbs

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gobicycle/bicycle/pkg/trust"
	"github.com/xssnick/tonutils-go/ton"
)

type Orbs struct {
}

func (Orbs) CheckBlock(id ton.BlockIDExt) error {
	r, err := http.Get(fmt.Sprintf("https://ton.access.orbs.network/route/1/mainnet/toncenter-api-v2/lookupBlock?workchain=%v&shard=%x&seqno=%v", id.Workchain, uint64(id.Shard), id.SeqNo))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", r.Status)
	}
	var resp struct {
		Result struct {
			RootHash string `json:"root_hash"`
			FileHash string `json:"file_hash"`
		} `json:"result"`
	}
	err = json.NewDecoder(r.Body).Decode(&resp)
	if err != nil {
		return err
	}
	rootHash, err := base64.StdEncoding.DecodeString(resp.Result.RootHash)
	if err != nil {
		return err
	}
	fileHash, err := base64.StdEncoding.DecodeString(resp.Result.FileHash)
	if err != nil {
		return err
	}
	if !bytes.Equal(rootHash, id.RootHash) || !bytes.Equal(fileHash, id.FileHash) {
		return trust.MismatchHead
	}
	return nil
}
