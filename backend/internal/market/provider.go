// Package market defines the provider boundary. Implementations must remain
// outside the strategy engine so providers can be exchanged later.
package market

import "time"

type Candle struct {
	Timestamp time.Time `json:"timestamp"`
	Open      float64   `json:"open"`
	High      float64   `json:"high"`
	Low       float64   `json:"low"`
	Close     float64   `json:"close"`
	Volume    float64   `json:"volume"`
}

type Snapshot struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type DataProvider interface {
	GetCurrentPrice(symbol string) (float64, error)
	GetDailyCandles(symbol string, limit int) ([]Candle, error)
}
