package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_API_ADDR", "")
	t.Setenv("CRYPTO_ASSISTANT_DB_PATH", "")
	t.Setenv("CRYPTO_ASSISTANT_MIGRATIONS_DIR", "")

	cfg := Load()
	if cfg.APIAddr != ":8080" || cfg.DatabasePath != "data/cryptoassistant.db" || cfg.MigrationsDir != "migrations" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
