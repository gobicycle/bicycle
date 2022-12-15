package main

import (
	"context"
	"errors"
	"github.com/gobicycle/bicycle/api"
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/config"
	"github.com/gobicycle/bicycle/core"
	"github.com/gobicycle/bicycle/db"
	"github.com/gobicycle/bicycle/queue"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	config.GetConfig()

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)
	wg := new(sync.WaitGroup)

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

	isTimeSynced, err := bcClient.CheckTime(ctx, config.AllowableServiceToNodeTimeDiff)
	if err != nil {
		log.Fatalf("get node time err: %v", err)
	}
	if !isTimeSynced {
		log.Fatalf("Service and Node time not synced")
	}

	wallets, err := core.InitWallets(ctx, dbClient, bcClient, config.Config.Seed, config.Config.Jettons)
	if err != nil {
		log.Fatalf("Hot wallets initialization error: %v", err)
	}

	queueClient, err := queue.NewAmqpClient(config.Config.QueueURI, config.Config.QueueEnabled, config.Config.QueueName)
	if err != nil {
		log.Fatalf("Create new queue client error: %v", err)
	}

	var tracker *blockchain.ShardTracker
	block, err := dbClient.GetLastSavedBlockID(ctx)
	if !errors.Is(err, core.ErrNotFound) && err != nil {
		log.Fatalf("Get last saved block error: %v", err)
	} else if errors.Is(err, core.ErrNotFound) {
		tracker = blockchain.NewShardTracker(wallets.Shard, nil, bcClient)
	} else {
		tracker = blockchain.NewShardTracker(wallets.Shard, block, bcClient)
	}

	blockScanner := core.NewBlockScanner(wg, dbClient, bcClient, wallets.Shard, tracker, queueClient)

	withdrawalsProcessor := core.NewWithdrawalsProcessor(
		wg, dbClient, bcClient, wallets, config.Config.ColdWallet)
	withdrawalsProcessor.Start()

	apiMux := http.NewServeMux()
	h := api.NewHandler(dbClient, bcClient, config.Config.APIToken, wallets.Shard, *wallets.TonHotWallet.Address())
	api.RegisterHandlers(apiMux, h)
	go func() {
		err := http.ListenAndServe(config.Config.APIHost, apiMux)
		if err != nil {
			log.Fatalf("api error: %v", err)
		}
	}()

	go func() {
		<-sigChannel
		log.Printf("SIGTERM received")
		blockScanner.Stop()
		withdrawalsProcessor.Stop()
	}()

	wg.Wait()
}
