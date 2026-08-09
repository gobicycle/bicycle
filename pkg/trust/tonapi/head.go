package tonapi

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gobicycle/bicycle/pkg/trust"
	"github.com/xssnick/tonutils-go/ton"
)

type Tonapi struct {
}

func (Tonapi) CheckBlock(id ton.BlockIDExt) error {
	r, err := http.Get(fmt.Sprintf("https://tonapi.io/v2/blockchain/blocks/(%v,%x,%v)", id.Workchain, uint64(id.Shard), id.SeqNo))
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("tonapi bad status: %s", r.Status)
	}
	var block struct {
		RootHash string `json:"root_hash"`
		FileHash string `json:"file_hash"`
	}

	err = json.NewDecoder(r.Body).Decode(&block)
	if err != nil {
		return err
	}
	if strings.ToLower(block.RootHash) != hex.EncodeToString(id.RootHash) || strings.ToLower(block.FileHash) != hex.EncodeToString(id.FileHash) {
		return trust.MismatchHead
	}
	return nil
}
