package api

import (
	"context"
	"crypto/subtle"
	"errors"
	"github.com/gobicycle/bicycle/internal/oas"
)

var ErrInvalidToken = errors.New("invalid token")

type SecurityHandler struct {
	token string
}

func (h *SecurityHandler) HandleBearerAuth(ctx context.Context, operationName string, t oas.BearerAuth) (context.Context, error) {
	if x := subtle.ConstantTimeCompare([]byte(t.GetToken()), []byte(h.token)); x != 1 {
		return nil, ErrInvalidToken
	} // constant time comparison to prevent time attack
	return ctx, nil
}

func NewSecurityHandler(token string) *SecurityHandler {
	return &SecurityHandler{token: token}
}
