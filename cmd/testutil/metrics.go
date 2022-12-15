package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	payerWalletBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "payer_wallet_balance",
			Help: "Payer wallet balance",
		},
		[]string{"currency"},
	)
	hotWalletBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hot_wallet_balance",
			Help: "Hot wallet balance",
		},
		[]string{"currency"},
	)
	depositWalletBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "deposit_wallet_balance",
			Help: "Deposit wallet balance",
		},
		[]string{"currency", "address"},
	)
	totalBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_balance",
			Help: "Total balance",
		},
		[]string{"currency"},
	)
	totalLosses = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_losses",
			Help: "Total losses",
		},
		[]string{"currency"},
	)
)
