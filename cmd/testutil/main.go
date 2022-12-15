package main

import (
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/config"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
	"os"
	"time"
)

var (
	onlyMonitoring = true
	payerSeed      string
)

const (
	depositsQty    = 3
	payAmount      = 1_000_000_000
	withdrawCutoff = 30_000_000_000
	//withdrawAmount = 500_000_000
)

func main() {

	config.GetConfig()
	if circulation := os.Getenv("CIRCULATION"); circulation == "true" {
		onlyMonitoring = false
	}

	if payerSeed = os.Getenv("PAYER_SEED"); payerSeed == "" {
		log.Fatalf("payer seed env variable empty")
	}

	bcClient, err := blockchain.NewConnection(config.Config.LiteServer, config.Config.LiteServerKey)
	if err != nil {
		log.Fatalf("blockchain connection error: %v", err)
	}

	//dbClient, err := db.NewConnection(config.Config.DatabaseURI)
	//if err != nil {
	//	log.Fatalf("DB connection error: %v", err)
	//}
	//_ = dbClient

	httpClient := NewClient(config.Config.APIHost, config.Config.APIToken, "TestClient")

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		log.Fatal(http.ListenAndServe(":9101", nil))
	}()

	deposits, err := httpClient.InitDeposits()
	if err != nil {
		log.Fatalf("can not init deposits: %v", err)
	}

	payer, _, _, err := bcClient.GenerateDefaultWallet(payerSeed, true)
	if err != nil {
		log.Fatalf("can not init payer wallet: %v", err)
	}
	log.Printf("Payer address: %v\n", payer.Address())

	hot, _, _, err := bcClient.GenerateDefaultWallet(config.Config.Seed, true)
	if err != nil {
		log.Fatalf("can not init hot wallet: %v", err)
	}
	log.Printf("Hot address: %v\n", hot.Address())

	payerProc := NewPayerProcessor(httpClient, deposits, payer, hot, bcClient)
	payerProc.Start()

	for {
		time.Sleep(time.Hour)
	}
}
