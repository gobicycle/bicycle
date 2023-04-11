package api

import (
	"encoding/json"
	"fmt"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// Deprecated
type balance struct {
	Address  string `json:"address"`
	Balance  string `json:"balance"`
	Currency string `json:"currency"`
}

// Deprecated: GetBalance method renamed to GetIncome
type GetBalanceResponse struct {
	Side     string    `json:"counting_side"`
	Balances []balance `json:"balances"`
}

// Deprecated
func convertBalance(dbConn storage, totalIncomes []core.TotalIncome) GetBalanceResponse {
	var res = GetBalanceResponse{
		Balances: []balance{},
	}
	if config.Config.IsDepositSideCalculation {
		res.Side = core.SideDeposit
	} else {
		res.Side = core.SideHotWallet
	}

	for _, b := range totalIncomes {
		bal := balance{
			Balance:  b.Amount.String(),
			Currency: b.Currency,
		}
		if b.Currency == core.TonSymbol {
			bal.Address = b.Deposit.ToUserFormat()
		} else {
			owner := dbConn.GetOwner(b.Deposit)
			if owner == nil {
				// TODO: remove fatal
				log.Fatalf("can not find owner for deposit: %s", b.Deposit.ToUserFormat())
			}
			bal.Address = owner.ToUserFormat()
		}
		res.Balances = append(res.Balances, bal)
	}
	return res
}

// Deprecated: getBalance is replaced by getTotalIncome
func (h *Handler) getBalance(resp http.ResponseWriter, req *http.Request) {
	id := req.URL.Query().Get("user_id")
	if id == "" {
		writeHttpError(resp, http.StatusBadRequest, "need to provide user ID")
		return
	}
	totalIncomes, err := h.storage.GetIncome(req.Context(), id, config.Config.IsDepositSideCalculation)
	if err != nil {
		writeHttpError(resp, http.StatusInternalServerError, fmt.Sprintf("get balances err: %v", err))
		return
	}
	resp.Header().Add("Content-Type", "application/json")
	resp.WriteHeader(http.StatusOK)
	err = json.NewEncoder(resp).Encode(convertBalance(h.storage, totalIncomes))
	if err != nil {
		log.Errorf("json encode error: %v", err)
	}
}
