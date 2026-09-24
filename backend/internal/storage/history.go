package storage

import (
	"context"
	"time"
)

type StrategyEvent struct {
	ID        int64     `json:"id"`
	Asset     string    `json:"asset"`
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Price     float64   `json:"price"`
	Reason    string    `json:"reason"`
	State     string    `json:"state"`
}

func (s *Store) ListStrategyEvents(ctx context.Context, limit int) ([]StrategyEvent, error) {
	if limit < 1 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, asset, timestamp, action, price, reason, state FROM strategy_events ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]StrategyEvent, 0)
	for rows.Next() {
		var event StrategyEvent
		var timestamp string
		if err := rows.Scan(&event.ID, &event.Asset, &timestamp, &event.Action, &event.Price, &event.Reason, &event.State); err != nil {
			return nil, err
		}
		var err error
		event.Timestamp, err = parseDatabaseTime(timestamp)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}
