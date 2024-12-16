package main

import (
	"context"
	"fmt"
	"github.com/gobicycle/bicycle/api"
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/gobicycle/bicycle/db"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

var Version = "dev"

func main() {

	log.Infof("App version: %s", Version)

	config.GetConfig()

	bcClient, err := blockchain.NewConnection(config.Config.LiteServer, config.Config.LiteServerKey)
	if err != nil {
		log.Fatalf("blockchain connection error: %v", err)
	}

	dbClient, err := db.NewConnection(config.Config.DatabaseURI)
	if err != nil {
		log.Fatalf("DB connection error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	err = dbClient.LoadAddressBook(ctx)
	if err != nil {
		log.Fatalf("address book loading error: %v", err)
	}

	hotWalletAddr, err := dbClient.GetTonHotWalletAddress(ctx)
	if err != nil {
		log.Fatalf("load hot wallet error: %v", err)
	}

	tonHotWallet, shard, _, err := bcClient.GenerateDefaultWallet(config.Config.Seed, true)
	if err != nil {
		log.Fatalf("generate hot wallet error: %v", err)
	}

	if core.AddressMustFromTonutilsAddress(tonHotWallet.Address()) != hotWalletAddr {
		log.Fatalf("saved hot wallet not equal generated hot wallet. Maybe seed was being changed")
	}

	apiMux := http.NewServeMux()
	h := api.NewHandler(dbClient, bcClient, config.Config.APIToken, shard, *tonHotWallet.Address())
	api.RegisterHandlers(apiMux, h)

	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Config.APIPort), apiMux)
	if err != nil {
		log.Fatalf("api error: %v", err)
	}
}
