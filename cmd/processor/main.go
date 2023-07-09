package main

import (
	"context"
	"errors"
	"github.com/gobicycle/bicycle/blockchain"
	"github.com/gobicycle/bicycle/internal/api"
	blockchain2 "github.com/gobicycle/bicycle/internal/blockchain"
	"github.com/gobicycle/bicycle/internal/config"
	core2 "github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/database"
	"github.com/gobicycle/bicycle/internal/queue"
	"github.com/gobicycle/bicycle/internal/webhook"
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

	bcClient, err := blockchain2.NewConnection(config.Config.LiteServer, config.Config.LiteServerKey)
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

	wallets, err := core2.InitWallets(ctx, dbClient, bcClient, config.Config.Seed, config.Config.Jettons, config.Config.ShardDepth)
	if err != nil {
		log.Fatalf("Hot wallets initialization error: %v", err)
	}

	var notificators []core2.Notificator

	if config.Config.QueueEnabled {
		queueClient, err := queue.NewAmqpClient(config.Config.QueueURI, config.Config.QueueEnabled, config.Config.QueueName)
		if err != nil {
			log.Fatalf("new queue client creating error: %v", err)
		}
		notificators = append(notificators, queueClient)
	}

	if config.Config.WebhookEndpoint != "" {
		webhookClient, err := webhook.NewWebhookClient(config.Config.WebhookEndpoint, config.Config.WebhookToken)
		if err != nil {
			log.Fatalf("new webhook client creating error: %v", err)
		}
		notificators = append(notificators, webhookClient)
	}

	var tracker *blockchain2.ShardTracker
	block, err := dbClient.GetLastSavedBlockID(ctx)
	if !errors.Is(err, core2.ErrNotFound) && err != nil {
		log.Fatalf("Get last saved block error: %v", err)
	} else if errors.Is(err, core2.ErrNotFound) {
		tracker, err = blockchain2.NewShardTracker(bcClient, blockchain2.WithShard(wallets.Shard))
	} else {
		tracker, err = blockchain2.NewShardTracker(bcClient, blockchain2.WithShard(wallets.Shard), blockchain.WithStartBlock(block))
	}
	if err != nil {
		log.Fatalf("shard tracker creating error: %v", err)
	}

	blockScanner := core2.NewBlockScanner(wg, dbClient, bcClient, wallets.Shard, tracker, notificators)

	withdrawalsProcessor := core2.NewWithdrawalsProcessor(
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
