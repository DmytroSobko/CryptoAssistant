package storage

import (
	"context"
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

// LatestSnapshot reads the most recently persisted data point. A concrete
// market provider will populate this table in the market-data phase.
func (s *Store) LatestSnapshot(ctx context.Context, symbol string) (market.Snapshot, error) {
	var snapshot market.Snapshot
	var updatedAt string
	err := s.db.QueryRowContext(ctx, `SELECT asset, price, timestamp FROM market_snapshots WHERE asset = ? ORDER BY timestamp DESC LIMIT 1`, symbol).Scan(&snapshot.Symbol, &snapshot.Price, &updatedAt)
	if err != nil {
		return market.Snapshot{}, fmt.Errorf("get latest %s snapshot: %w", symbol, err)
	}
	snapshot.UpdatedAt, err = parseDatabaseTime(updatedAt)
	if err != nil {
		return market.Snapshot{}, fmt.Errorf("parse latest %s timestamp: %w", symbol, err)
	}
	return snapshot, nil
}
