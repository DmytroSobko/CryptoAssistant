package storage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestPriceRefreshIntervalUsesFallbackAndPersistsOverride(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}

	if seconds, err := store.PriceRefreshIntervalSeconds(context.Background(), 30); err != nil || seconds != 30 {
		t.Fatalf("initial interval = %d, err = %v", seconds, err)
	}
	if err := store.SavePriceRefreshIntervalSeconds(context.Background(), 12); err != nil {
		t.Fatalf("save interval: %v", err)
	}
	if seconds, err := store.PriceRefreshIntervalSeconds(context.Background(), 30); err != nil || seconds != 12 {
		t.Fatalf("saved interval = %d, err = %v", seconds, err)
	}
	if err := store.SavePriceRefreshIntervalSeconds(context.Background(), 0); err == nil {
		t.Fatal("expected zero-second interval to be rejected")
	}
	if err := store.SavePriceRefreshIntervalSeconds(context.Background(), MaxPriceRefreshIntervalSeconds+1); err == nil {
		t.Fatal("expected oversized interval to be rejected")
	}
}
