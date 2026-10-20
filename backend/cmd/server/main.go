package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/api"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtestservice"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market/coinbase"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/marketdata"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

func main() {
	cfg := config.Load()
	store, err := storage.Open(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer store.Close()

	if err := store.Migrate(cfg.MigrationsDir); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	provider := coinbase.NewProvider(nil, cfg.MarketDataBaseURL)
	refresher := marketdata.NewRefresher(provider, store)
	managedBacktestData := backtestservice.NewManagedHistory(store, provider)
	refreshContext, cancelRefresh := context.WithCancel(context.Background())
	defer cancelRefresh()
	go refreshMarketData(refreshContext, refresher, cfg.MarketRefreshInterval)
	go initializeBacktestData(refreshContext, managedBacktestData)

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           api.NewServerWithManagedBacktestData(store, cfg, refresher, managedBacktestData).Routes(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("API listening on http://localhost%s", cfg.APIAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve API: %v", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
	<-shutdown

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("shutdown API: %v", err)
	}
}

func initializeBacktestData(ctx context.Context, managed *backtestservice.ManagedHistory) {
	if err := managed.Initialize(ctx); err != nil {
		log.Printf("initialize managed backtest data: %v", err)
	}
}

func refreshMarketData(ctx context.Context, refresher *marketdata.Refresher, interval time.Duration) {
	refresh := func() {
		refreshContext, cancel := context.WithTimeout(ctx, 70*time.Second)
		defer cancel()
		if err := refresher.RefreshAll(refreshContext); err != nil {
			log.Printf("refresh public market data: %v", err)
		}
	}
	refresh()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refresh()
		}
	}
}
