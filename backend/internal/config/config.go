// Package config loads runtime configuration. Strategy configuration belongs in
// SQLite; these values only configure the local process and infrastructure.
package config

import "os"

type Config struct {
	APIAddr       string
	DatabasePath  string
	MigrationsDir string
}

func Load() Config {
	return Config{
		APIAddr:       envOr("CRYPTO_ASSISTANT_API_ADDR", ":8080"),
		DatabasePath:  envOr("CRYPTO_ASSISTANT_DB_PATH", "data/cryptoassistant.db"),
		MigrationsDir: envOr("CRYPTO_ASSISTANT_MIGRATIONS_DIR", "migrations"),
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
