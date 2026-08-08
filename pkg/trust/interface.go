package trust

import (
	"errors"

	"github.com/xssnick/tonutils-go/ton"
)

var MismatchHead = errors.New("mismatched block hash. IMPORTANT ERROR. someone is trying to scam you.")

type Trust interface {
	CheckBlock(id ton.BlockIDExt) error
}
