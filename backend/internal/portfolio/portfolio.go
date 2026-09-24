// Package portfolio owns domain types for the user's manually managed assets.
package portfolio

import "time"

type Asset struct {
	Symbol            string    `json:"symbol"`
	Quantity          float64   `json:"quantity"`
	AverageEntryPrice float64   `json:"averageEntryPrice"`
	CashAllocated     float64   `json:"cashAllocated"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

type Portfolio struct {
	CashBalance float64 `json:"cashBalance"`
	Assets      []Asset `json:"assets"`
}

type Transaction struct {
	ID        int64     `json:"id"`
	Asset     string    `json:"asset"`
	Type      string    `json:"type"`
	Quantity  float64   `json:"quantity"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
	Notes     string    `json:"notes"`
}
