package backtestservice

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtestdata"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

const coinbaseHistoryPageDays = 300

var managedHistoryAssets = []string{"BTC", "ETH"}

type DailyCandleRangeProvider interface {
	GetDailyCandlesRange(symbol string, start, end time.Time) ([]market.Candle, error)
}

type HistoryRefreshResult struct {
	CandleSet    storage.BacktestCandleSet
	AddedCandles int
}

// ManagedHistory owns one append-only Coinbase daily series per asset in
// SQLite. Its per-asset locks serialize launch backfills, UI reads, and manual
// refreshes without blocking the other currency.
type ManagedHistory struct {
	store    *storage.Store
	provider DailyCandleRangeProvider
	now      func() time.Time
	locks    map[string]*sync.Mutex
}

func NewManagedHistory(store *storage.Store, provider DailyCandleRangeProvider) *ManagedHistory {
	return &ManagedHistory{
		store: store, provider: provider, now: time.Now,
		locks: map[string]*sync.Mutex{"BTC": {}, "ETH": {}},
	}
}

func (h *ManagedHistory) Initialize(ctx context.Context) error {
	var failures []error
	for _, asset := range managedHistoryAssets {
		if _, err := h.Refresh(ctx, asset); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", asset, err))
		}
	}
	return errors.Join(failures...)
}

// Current returns an up-to-date managed set, performing the initial backfill
// or any small missed append itself if startup initialization has not finished.
func (h *ManagedHistory) Current(ctx context.Context, asset string) (storage.BacktestCandleSet, error) {
	asset, err := normalizeHistoryAsset(asset)
	if err != nil {
		return storage.BacktestCandleSet{}, err
	}
	lock := h.locks[asset]
	lock.Lock()
	defer lock.Unlock()
	result, err := h.refreshUnlocked(ctx, asset)
	return result.CandleSet, err
}

func (h *ManagedHistory) Refresh(ctx context.Context, asset string) (HistoryRefreshResult, error) {
	asset, err := normalizeHistoryAsset(asset)
	if err != nil {
		return HistoryRefreshResult{}, err
	}
	lock := h.locks[asset]
	lock.Lock()
	defer lock.Unlock()
	return h.refreshUnlocked(ctx, asset)
}

func (h *ManagedHistory) refreshUnlocked(ctx context.Context, asset string) (HistoryRefreshResult, error) {
	if h.store == nil || h.provider == nil {
		return HistoryRefreshResult{}, fmt.Errorf("managed historical data is unavailable")
	}
	set, err := h.store.GetBacktestCandleSetMetadata(ctx, managedHistoryID(asset))
	isNew := errors.Is(err, sql.ErrNoRows)
	if err != nil && !isNew {
		return HistoryRefreshResult{}, err
	}

	end := h.now().UTC().Truncate(24 * time.Hour)
	start := time.Date(2015, time.January, 1, 0, 0, 0, 0, time.UTC)
	if !isNew {
		start = set.LastTimestamp.AddDate(0, 0, 1)
	}
	if !start.Before(end) {
		return HistoryRefreshResult{CandleSet: set}, nil
	}

	added := make([]market.Candle, 0)
	for cursor := start; cursor.Before(end); {
		if err := ctx.Err(); err != nil {
			return HistoryRefreshResult{}, err
		}
		pageEnd := cursor.AddDate(0, 0, coinbaseHistoryPageDays)
		if pageEnd.After(end) {
			pageEnd = end
		}
		page, err := h.provider.GetDailyCandlesRange(asset, cursor, pageEnd)
		if err != nil {
			return HistoryRefreshResult{}, fmt.Errorf("load %s candles from %s: %w", asset, cursor.Format("2006-01-02"), err)
		}
		added = append(added, page...)
		cursor = pageEnd
	}
	if len(added) == 0 && !isNew {
		return HistoryRefreshResult{CandleSet: set}, nil
	}
	if isNew {
		// Coinbase does not publish buckets for intervals with no trades. Early
		// ETH history contains such gaps, so retain the longest contiguous
		// suffix ending at the latest completed day: this is the maximum usable
		// daily series for an SMA-based backtest without fabricating candles.
		added = longestContiguousSuffix(added)
	}
	existing := make([]market.Candle, 0)
	if !isNew {
		_, existing, err = h.store.GetBacktestCandleSet(ctx, managedHistoryID(asset))
		if err != nil {
			return HistoryRefreshResult{}, err
		}
	}
	merged := append(append([]market.Candle(nil), existing...), added...)
	if len(merged) < backtest.MinimumCandleCount {
		return HistoryRefreshResult{}, fmt.Errorf("Coinbase returned only %d %s candles; at least %d are required", len(merged), asset, backtest.MinimumCandleCount)
	}
	if err := backtest.ValidateCandles(merged); err != nil {
		return HistoryRefreshResult{}, fmt.Errorf("Coinbase %s history is not contiguous: %w", asset, err)
	}
	encoded, err := backtestdata.EncodeCSV(merged)
	if err != nil {
		return HistoryRefreshResult{}, err
	}
	digest := backtestdata.SHA256(append([]byte("managed:"+asset+"\n"), encoded...))
	if isNew {
		set, err = h.store.ReplaceManagedBacktestCandleSet(ctx, storage.BacktestCandleSet{
			Asset: asset, SourceLabel: "Coinbase Exchange API (managed)",
			SourceFilename: strings.ToLower(asset) + "-usd-daily", OriginalSHA256: digest,
		}, merged)
	} else {
		set, err = h.store.AppendManagedBacktestCandles(ctx, asset, added, digest)
	}
	if err != nil {
		return HistoryRefreshResult{}, err
	}
	return HistoryRefreshResult{CandleSet: set, AddedCandles: len(added)}, nil
}

func normalizeHistoryAsset(asset string) (string, error) {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	if asset != "BTC" && asset != "ETH" {
		return "", fmt.Errorf("asset must be BTC or ETH")
	}
	return asset, nil
}

func managedHistoryID(asset string) string { return "managed-" + strings.ToLower(asset) }

func longestContiguousSuffix(candles []market.Candle) []market.Candle {
	start := 0
	for index := 1; index < len(candles); index++ {
		if candles[index].Timestamp.Sub(candles[index-1].Timestamp) != 24*time.Hour {
			start = index
		}
	}
	return candles[start:]
}
