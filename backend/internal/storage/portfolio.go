package storage

import (
	"context"
	"fmt"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

func (s *Store) GetPortfolio(ctx context.Context) (portfolio.Portfolio, error) {
	result := portfolio.Portfolio{}
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'cash_balance'`).Scan(&result.CashBalance); err != nil {
		return result, fmt.Errorf("get cash balance: %w", err)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT symbol, quantity, average_entry_price, cash_allocated, updated_at FROM portfolio ORDER BY symbol`)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var asset portfolio.Asset
		var updatedAt string
		if err := rows.Scan(&asset.Symbol, &asset.Quantity, &asset.AverageEntryPrice, &asset.CashAllocated, &updatedAt); err != nil {
			return result, err
		}
		asset.UpdatedAt, err = parseDatabaseTime(updatedAt)
		if err != nil {
			return result, err
		}
		result.Assets = append(result.Assets, asset)
	}
	return result, rows.Err()
}

func (s *Store) SavePortfolio(ctx context.Context, input portfolio.Portfolio) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO settings(key, value) VALUES ('cash_balance', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, input.CashBalance); err != nil {
		return fmt.Errorf("save cash balance: %w", err)
	}
	for _, asset := range input.Assets {
		if asset.Symbol != "BTC" && asset.Symbol != "ETH" {
			return fmt.Errorf("unsupported asset %q", asset.Symbol)
		}
		if asset.Quantity < 0 || asset.AverageEntryPrice < 0 || asset.CashAllocated < 0 {
			return fmt.Errorf("portfolio values cannot be negative")
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO portfolio(symbol, quantity, average_entry_price, cash_allocated) VALUES (?, ?, ?, ?) ON CONFLICT(symbol) DO UPDATE SET quantity = excluded.quantity, average_entry_price = excluded.average_entry_price, cash_allocated = excluded.cash_allocated, updated_at = CURRENT_TIMESTAMP`, asset.Symbol, asset.Quantity, asset.AverageEntryPrice, asset.CashAllocated); err != nil {
			return fmt.Errorf("save %s portfolio: %w", asset.Symbol, err)
		}
	}
	return tx.Commit()
}
