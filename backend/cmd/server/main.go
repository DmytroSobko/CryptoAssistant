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
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
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

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           api.NewServer(store, cfg).Routes(),
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
