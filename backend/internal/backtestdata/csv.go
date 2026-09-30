// Package backtestdata validates the v1 historical-candle CSV interchange
// format. It has no database, HTTP, or provider dependency.
package backtestdata

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/backtest"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

var canonicalHeader = []string{"timestamp", "open", "high", "low", "close", "volume"}

// ParseCSV accepts only the documented, headered canonical schema. The
// returned candles retain the source's chronological order after it has been
// proven strictly ascending with no missing UTC days.
func ParseCSV(data []byte) ([]market.Candle, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.FieldsPerRecord = len(canonicalHeader)
	header, err := reader.Read()
	if err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("CSV header is required")
		}
		return nil, fmt.Errorf("read CSV header: %w", err)
	}
	for index, expected := range canonicalHeader {
		if header[index] != expected {
			return nil, fmt.Errorf("CSV header must be exactly timestamp,open,high,low,close,volume")
		}
	}

	candles := make([]market.Candle, 0)
	for row := 2; ; row++ {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read CSV row %d: %w", row, err)
		}
		candle, err := parseRecord(record)
		if err != nil {
			return nil, fmt.Errorf("CSV row %d: %w", row, err)
		}
		candles = append(candles, candle)
	}
	if len(candles) < backtest.MinimumCandleCount {
		return nil, fmt.Errorf("CSV requires at least %d daily candles", backtest.MinimumCandleCount)
	}
	if err := backtest.ValidateCandles(candles); err != nil {
		return nil, fmt.Errorf("invalid CSV candles: %w", err)
	}
	return candles, nil
}

func parseRecord(record []string) (market.Candle, error) {
	if !strings.HasSuffix(record[0], "Z") {
		return market.Candle{}, fmt.Errorf("timestamp must use UTC Z notation")
	}
	timestamp, err := time.Parse(time.RFC3339, record[0])
	if err != nil {
		return market.Candle{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	if timestamp.Format(time.RFC3339) != record[0] {
		return market.Candle{}, fmt.Errorf("timestamp must be canonical RFC 3339 UTC midnight")
	}
	values := make([]float64, 5)
	for index, raw := range record[1:] {
		if raw == "" && index == 4 {
			continue // Volume is optional in the v1 canonical schema.
		}
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return market.Candle{}, fmt.Errorf("invalid %s value: %w", canonicalHeader[index+1], err)
		}
		values[index] = value
	}
	return market.Candle{Timestamp: timestamp, Open: values[0], High: values[1], Low: values[2], Close: values[3], Volume: values[4]}, nil
}

// SHA256 returns the provenance fingerprint of the original, unmodified CSV
// bytes. It is deliberately separate from the run's selected-candle digest.
func SHA256(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
