package config

import (
	"testing"
	"time"
)

func TestLoadUsesDefaults(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_API_ADDR", "")
	t.Setenv("CRYPTO_ASSISTANT_DB_PATH", "")
	t.Setenv("CRYPTO_ASSISTANT_MIGRATIONS_DIR", "")
	t.Setenv("CRYPTO_ASSISTANT_MARKET_DATA_BASE_URL", "")
	t.Setenv("CRYPTO_ASSISTANT_MARKET_REFRESH_INTERVAL", "")
	t.Setenv("CRYPTO_ASSISTANT_PRICE_REFRESH_INTERVAL", "")

	cfg := Load()
	if cfg.APIAddr != ":8080" || cfg.DatabasePath != "data/cryptoassistant.db" || cfg.MigrationsDir != "migrations" || cfg.MarketDataBaseURL != "https://api.exchange.coinbase.com" || cfg.MarketRefreshInterval.String() != "15m0s" || cfg.PriceRefreshInterval.String() != "30s" {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
}

func TestLoadUsesConfiguredPriceRefreshInterval(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_PRICE_REFRESH_INTERVAL", "12s")

	if interval := Load().PriceRefreshInterval; interval != 12*time.Second {
		t.Fatalf("price refresh interval = %s, want 12s", interval)
	}
}

func TestLoadRejectsSubsecondPriceRefreshInterval(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_PRICE_REFRESH_INTERVAL", "500ms")

	if interval := Load().PriceRefreshInterval; interval != 30*time.Second {
		t.Fatalf("price refresh interval = %s, want 30s fallback", interval)
	}
}

func TestLoadRejectsPriceRefreshIntervalOverOneDay(t *testing.T) {
	t.Setenv("CRYPTO_ASSISTANT_PRICE_REFRESH_INTERVAL", "25h")

	if interval := Load().PriceRefreshInterval; interval != 30*time.Second {
		t.Fatalf("price refresh interval = %s, want 30s fallback", interval)
	}
}
