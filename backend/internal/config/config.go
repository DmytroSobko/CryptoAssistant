// Package config loads runtime configuration. Strategy configuration belongs in
// SQLite; these values only configure the local process and infrastructure.
package config

import (
	"log"
	"os"
	"time"
)

type Config struct {
	APIAddr               string
	DatabasePath          string
	MigrationsDir         string
	MarketDataBaseURL     string
	MarketRefreshInterval time.Duration
	PriceRefreshInterval  time.Duration
}

func Load() Config {
	return Config{
		APIAddr:               envOr("CRYPTO_ASSISTANT_API_ADDR", ":8080"),
		DatabasePath:          envOr("CRYPTO_ASSISTANT_DB_PATH", "data/cryptoassistant.db"),
		MigrationsDir:         envOr("CRYPTO_ASSISTANT_MIGRATIONS_DIR", "migrations"),
		MarketDataBaseURL:     envOr("CRYPTO_ASSISTANT_MARKET_DATA_BASE_URL", "https://api.exchange.coinbase.com"),
		MarketRefreshInterval: durationEnvOr("CRYPTO_ASSISTANT_MARKET_REFRESH_INTERVAL", 15*time.Minute),
		PriceRefreshInterval:  durationEnvOrBetween("CRYPTO_ASSISTANT_PRICE_REFRESH_INTERVAL", 30*time.Second, time.Second, 24*time.Hour),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func durationEnvOr(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 {
		log.Printf("invalid %s=%q; using %s", key, value, fallback)
		return fallback
	}
	return duration
}

func durationEnvOrBetween(key string, fallback, minimum, maximum time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration < minimum || duration > maximum {
		log.Printf("invalid %s=%q; using %s", key, value, fallback)
		return fallback
	}
	return duration
}
