// Package marketdata coordinates read-only provider calls with local market
// persistence. Keeping it separate preserves the strategy package's purity.
package marketdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
)

const DailyCandleLimit = 250

type Refresher struct {
	provider market.DataProvider
	store    *storage.Store
	now      func() time.Time
}

func NewRefresher(provider market.DataProvider, store *storage.Store) *Refresher {
	return &Refresher{provider: provider, store: store, now: time.Now}
}

// RefreshAsset first saves completed daily candles, then records the current
// display price. Strategy evaluation is deliberately not invoked here.
func (r *Refresher) RefreshAsset(ctx context.Context, symbol string) error {
	candles, err := r.provider.GetDailyCandles(symbol, DailyCandleLimit)
	if err != nil {
		return fmt.Errorf("load %s daily candles: %w", symbol, err)
	}
	if err := r.store.SaveDailyCandles(ctx, symbol, candles); err != nil {
		return fmt.Errorf("persist %s daily candles: %w", symbol, err)
	}
	price, err := r.provider.GetCurrentPrice(symbol)
	if err != nil {
		return fmt.Errorf("load %s current price: %w", symbol, err)
	}
	if err := r.store.SaveCurrentPrice(ctx, market.Snapshot{Symbol: symbol, Price: price, UpdatedAt: r.now().UTC()}); err != nil {
		return fmt.Errorf("persist %s current price: %w", symbol, err)
	}
	return nil
}

func (r *Refresher) RefreshAll(ctx context.Context) error {
	var refreshErrors []error
	for _, symbol := range []string{"BTC", "ETH"} {
		if err := r.RefreshAsset(ctx, symbol); err != nil {
			refreshErrors = append(refreshErrors, err)
		}
	}
	return errors.Join(refreshErrors...)
}
