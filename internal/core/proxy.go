package core

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/tonkeeper/tongo"
	"github.com/tonkeeper/tongo/boc"
	"github.com/tonkeeper/tongo/tlb"
	"github.com/tonkeeper/tongo/wallet"
)

// JettonProxyContractCode source code at https://github.com/gobicycle/ton-proxy-contract
const JettonProxyContractCode = "te6cckEBAgEANwABFP8A9KQT9LzyyAsBAFDTMzHQ0wMBcbCRW+D6QDDtRND6QDDHBfLhk5Mg10qX1AGBAKD7AOgwHoqQQA=="

// JettonProxy is a special contract wrapper that allow to control jetton wallet from TON wallet.
// It is possible create few jetton proxies for single TON wallet (as owner) and control multiple jetton wallets.
// Read about JettonProxy smart contract at README.md and https://github.com/gobicycle/ton-proxy-contract
type JettonProxy struct {
	Owner       tongo.AccountID
	SubwalletID uint32
	address     tongo.AccountID
	stateInit   *tlb.StateInit
}

type jettonProxyData struct {
	Owner       tlb.MsgAddress
	SubwalletID uint32
}

func NewJettonProxy(subwalletID uint32, owner tongo.AccountID) (*JettonProxy, error) {

	proxyData := jettonProxyData{
		Owner:       owner.ToMsgAddress(),
		SubwalletID: subwalletID,
	}
	stateInit := buildJettonProxyStateInit(proxyData)
	stateCell := boc.NewCell()

	err := tlb.Marshal(stateCell, stateInit)
	if err != nil {
		return nil, fmt.Errorf("failed to get state cell: %w", err)
	}

	addr, err := stateCell.Hash256()
	if err != nil {
		return nil, fmt.Errorf("failed to get hash for state init: %w", err)
	}

	proxyAccountID := tongo.NewAccountId(DefaultWorkchainID, addr)

	return &JettonProxy{
		Owner:       owner,
		SubwalletID: subwalletID,
		address:     *proxyAccountID,
		stateInit:   stateInit,
	}, nil
}

func buildJettonProxyStateInit(proxyData jettonProxyData) *tlb.StateInit {
	// TODO: test
	code, err := boc.DeserializeBocBase64(JettonProxyContractCode)
	if err != nil {
		log.Fatalf("parsing JettonProxyContractCode boc error: %v", err)
	}
	data := boc.NewCell()
	err = tlb.Marshal(data, proxyData)
	if err != nil {
		log.Fatalf("marshaling proxy data to boc error: %v", err)
	}
	res := &tlb.StateInit{
		Code: tlb.Maybe[tlb.Ref[boc.Cell]]{Exists: true, Value: tlb.Ref[boc.Cell]{Value: *code[0]}},
		Data: tlb.Maybe[tlb.Ref[boc.Cell]]{Exists: true, Value: tlb.Ref[boc.Cell]{Value: *data}},
	}
	return res
}

// Address returns address of jetton proxy contract
func (p *JettonProxy) Address() tongo.AccountID {
	return p.address
}

// Code returns code cell of jetton proxy contract
func (p *JettonProxy) Code() *boc.Cell {
	// TODO: or make new cell
	code := p.stateInit.Code.Value.Value
	return &code
}

// Data returns init (for StateInit) data cell of jetton proxy contract
func (p *JettonProxy) Data() *boc.Cell {
	// TODO: or make new cell
	data := p.stateInit.Data.Value.Value
	return &data
}

// BuildPayload wraps custom body payload to resend by proxy contract
func (p *JettonProxy) BuildPayload(destination tongo.AccountID, body *boc.Cell) (*boc.Cell, error) {

	msg := wallet.Message{
		Amount:  tlb.Coins(0),
		Address: destination,
		Body:    body,
		Bounce:  true,
		Mode:    0, // not used, proxy sends all TONs with mode == 128 + 32
	}

	internalMsg, _, err := msg.ToInternal()
	if err != nil { // impossible
		return nil, err
	}

	cell := boc.NewCell()
	err = tlb.Marshal(cell, internalMsg)
	if err != nil {
		return nil, err
	}

	return cell, nil
}
