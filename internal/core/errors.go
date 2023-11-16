package core

import "errors"

var (
	ErrSubwalletNotFound = errors.New("subwallet not found")      // TODO: clarify
	ErrJettonNotFound    = errors.New("jetton address not found") // TODO: clarify
	ErrNotFound          = errors.New("not found")
	ErrTimeoutExceeded   = errors.New("timeout exceeded")
)
