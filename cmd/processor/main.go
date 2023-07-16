package main

import (
	"context"
	"errors"
	"github.com/gobicycle/bicycle/internal/api"
	"github.com/gobicycle/bicycle/internal/app"
	"github.com/gobicycle/bicycle/internal/blockchain"
	"github.com/gobicycle/bicycle/internal/config"
	"github.com/gobicycle/bicycle/internal/core"
	"github.com/gobicycle/bicycle/internal/database"
	"github.com/gobicycle/bicycle/internal/queue"
	"github.com/gobicycle/bicycle/internal/webhook"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	cfg := config.Load()
	log := app.Logger(cfg.App.LogLevel)

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, os.Interrupt, syscall.SIGTERM)
	wg := new(sync.WaitGroup)

	bcClient, err := blockchain.NewConnection(cfg.Blockchain)
	if err != nil {
		log.Fatal("blockchain connection error", zap.Error(err))
	}

	dbClient, err := db.NewConnection(cfg.DB.URI)
	if err != nil {
		log.Fatal("DB connection error", zap.Error(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*120)
	defer cancel()

	err = dbClient.LoadAddressBook(ctx)
	if err != nil {
		log.Fatal("address book loading error", zap.Error(err))
	}

	isTimeSynced, err := bcClient.CheckTime(ctx, config.AllowableServiceToNodeTimeDiff)
	if err != nil {
		log.Fatal("get node time err", zap.Error(err))
	}
	if !isTimeSynced {
		log.Fatal("Service and Node time not synced")
	}

	wallets, err := core.InitWallets(ctx, dbClient, bcClient, cfg.Processor.Seed, cfg.Processor.Jettons, cfg.Blockchain.ShardDepth)
	if err != nil {
		log.Fatal("Hot wallets initialization error", zap.Error(err))
	}

	var notificators []core.Notificator

	if cfg.Queue.URI != "" {
		queueClient, err := queue.NewAmqpClient(cfg.Queue.URI, cfg.Queue.Name)
		if err != nil {
			log.Fatal("new queue client creating error", zap.Error(err))
		}
		notificators = append(notificators, queueClient)
	}

	if cfg.Webhook.Endpoint != "" {
		webhookClient, err := webhook.NewWebhookClient(cfg.Webhook.Endpoint, cfg.Webhook.Token)
		if err != nil {
			log.Fatal("new webhook client creating error", zap.Error(err))
		}
		notificators = append(notificators, webhookClient)
	}

	var tracker *blockchain.ShardTracker
	block, err := dbClient.GetLastSavedBlockID(ctx)
	if !errors.Is(err, core.ErrNotFound) && err != nil {
		log.Fatal("Get last saved block error", zap.Error(err))
	} else if errors.Is(err, core.ErrNotFound) {
		tracker, err = blockchain.NewShardTracker(bcClient, blockchain.WithShard(wallets.Shard))
	} else {
		tracker, err = blockchain.NewShardTracker(bcClient, blockchain.WithShard(wallets.Shard), blockchain.WithStartBlockSeqno(block))
	}
	if err != nil {
		log.Fatal("shard tracker creating error", zap.Error(err))
	}

	blockScanner := core.NewBlockScanner(wg, dbClient, bcClient, wallets.Shard, tracker, notificators)

	withdrawalsProcessor := core.NewWithdrawalsProcessor(
		wg, dbClient, bcClient, wallets, cfg.Processor.ColdWallet)
	withdrawalsProcessor.Start()

	h, err := api.NewHandler(log,
		api.WithStorage(dbClient),
		api.WithBlockchain(bcClient),
		api.WithShard(wallets.Shard),
	)
	if err != nil {
		log.Fatal("failed to create api handler", zap.Error(err))
	}

	server, err := api.NewServer(log, h, cfg.API.Token, cfg.API.Host)
	if err != nil {
		log.Fatal("failed to create server", zap.Error(err))
	}

	log.Info("start server", zap.String("host", cfg.API.Host))
	server.Run()

	go func() {
		<-sigChannel
		log.Info("SIGTERM received")
		blockScanner.Stop()
		withdrawalsProcessor.Stop()
	}()

	wg.Wait()
}
