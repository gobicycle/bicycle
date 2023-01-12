package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	hotWalletABalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hot_wallet_a_balance",
			Help: "Hot wallet A balance",
		},
		[]string{"currency"},
	)
	hotWalletBBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "hot_wallet_b_balance",
			Help: "Hot wallet B balance",
		},
		[]string{"currency"},
	)
	depositWalletABalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "deposit_wallet_a_balance",
			Help: "Deposit wallet A balance",
		},
		[]string{"currency", "address"},
	)
	depositWalletBBalance = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "deposit_wallet_b_balance",
			Help: "Deposit wallet B balance",
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
	predictedTonLoss = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "predicted_ton_loss",
			Help: "Predicted TON loss",
		},
	)
	totalProcessedAmount = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "total_processed_amount",
			Help: "Total processed amount",
		},
		[]string{"currency"},
	)
)
