package config

import "testing"

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_API_ADDR", "")
	t.Setenv("CRYPTO_ASSISTANT_DB_PATH", "")
	t.Setenv("CRYPTO_ASSISTANT_MIGRATIONS_DIR", "")
	t.Setenv("CRYPTO_ASSISTANT_MARKET_DATA_BASE_URL", "")
	t.Setenv("CRYPTO_ASSISTANT_MARKET_REFRESH_INTERVAL", "")

	cfg := Load()
	if cfg.APIAddr != ":8080" || cfg.DatabasePath != "data/cryptoassistant.db" || cfg.MigrationsDir != "migrations" || cfg.MarketDataBaseURL != "https://api.exchange.coinbase.com" || cfg.MarketRefreshInterval.String() != "15m0s" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}
