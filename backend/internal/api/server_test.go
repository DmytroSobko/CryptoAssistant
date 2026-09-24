package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/config"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

func newTestHandler(t *testing.T) http.Handler {
	t.Helper()
	store, err := storage.Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate store: %v", err)
	}
	return NewServer(store, config.Config{}).Routes()
}

func TestPortfolioAndConfigEndpoints(t *testing.T) {
	handler := newTestHandler(t)

	put := httptest.NewRequest(http.MethodPut, "/api/portfolio", bytes.NewBufferString(`{"cashBalance":2500,"assets":[{"symbol":"BTC","quantity":0.12,"averageEntryPrice":71200,"cashAllocated":8544}]}`))
	put.Header.Set("Content-Type", "application/json")
	putResponse := httptest.NewRecorder()
	handler.ServeHTTP(putResponse, put)
	if putResponse.Code != http.StatusNoContent {
		t.Fatalf("save portfolio status = %d, body = %s", putResponse.Code, putResponse.Body.String())
	}

	getResponse := httptest.NewRecorder()
	handler.ServeHTTP(getResponse, httptest.NewRequest(http.MethodGet, "/api/portfolio", nil))
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get portfolio status = %d, body = %s", getResponse.Code, getResponse.Body.String())
	}
	if !bytes.Contains(getResponse.Body.Bytes(), []byte(`"cashBalance":2500`)) || !bytes.Contains(getResponse.Body.Bytes(), []byte(`"symbol":"BTC"`)) {
		t.Fatalf("unexpected portfolio body: %s", getResponse.Body.String())
	}

	configResponse := httptest.NewRecorder()
	handler.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if configResponse.Code != http.StatusOK || !bytes.Contains(configResponse.Body.Bytes(), []byte(`"pullbackMinPct":10`)) {
		t.Fatalf("unexpected config response (%d): %s", configResponse.Code, configResponse.Body.String())
	}
}
