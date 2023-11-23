package core

import (
	"github.com/google/uuid"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
)

type tonFillMessage struct {
	To     tongo.AccountID
	Amount tlb.Coins
	Memo   uuid.UUID
}

func (m tonFillMessage) ToInternal() (tlb.Message, uint8, error) {
	return wallet.Message{
		Amount:  m.Amount,
		Address: m.To,
		Body:    buildComment(m.Memo.String()),
		Bounce:  false,
		Mode:    3,
	}.ToInternal()
}
