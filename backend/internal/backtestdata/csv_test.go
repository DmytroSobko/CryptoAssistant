package backtestdata

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

func TestParseCSVAcceptsCanonicalDailyData(t *testing.T) {
	data := csvFixture(candleFixture(201))
	candles, err := ParseCSV([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 201 || !candles[0].Timestamp.Equal(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)) || candles[0].Volume != 1 {
		t.Fatalf("unexpected parsed candles: first=%+v count=%d", candles[0], len(candles))
	}
	if SHA256([]byte(data)) != SHA256([]byte(data)) || SHA256([]byte(data)) == SHA256([]byte(data+"\n")) {
		t.Fatal("SHA-256 must fingerprint the exact original bytes")
	}
}

func TestParseCSVRejectsInvalidSchemaAndCandleData(t *testing.T) {
	base := candleFixture(201)
	tests := []struct {
		name string
		data string
	}{
		{"header mismatch", strings.Replace(csvFixture(base), "timestamp,open", "date,open", 1)},
		{"too few candles", csvFixture(base[:200])},
		{"non UTC timestamp", strings.Replace(csvFixture(base), "2020-01-01T00:00:00Z", "2020-01-01T00:00:00+00:00", 1)},
		{"duplicate day", csvFixture(withTimestamp(base, 1, base[0].Timestamp))},
		{"missing day", csvFixture(withTimestamp(base, 1, base[1].Timestamp.AddDate(0, 0, 1)))},
		{"invalid OHLC", csvFixture(withHigh(base, 2, 1))},
		{"negative value", strings.Replace(csvFixture(base), ",100,101,99,100,1", ",-100,101,99,100,1", 1)},
		{"not a number", strings.Replace(csvFixture(base), ",100,101,99,100,1", ",NaN,101,99,100,1", 1)},
		{"unsorted", csvFixture(swapped(base, 0, 1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := ParseCSV([]byte(test.data)); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func candleFixture(count int) []market.Candle {
	candles := make([]market.Candle, count)
	for index := range candles {
		price := float64(100 + index)
		candles[index] = market.Candle{Timestamp: time.Date(2020, 1, 1+index, 0, 0, 0, 0, time.UTC), Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 1}
	}
	return candles
}

func csvFixture(candles []market.Candle) string {
	var builder strings.Builder
	builder.WriteString("timestamp,open,high,low,close,volume\n")
	for _, candle := range candles {
		fmt.Fprintf(&builder, "%s,%g,%g,%g,%g,%g\n", candle.Timestamp.Format(time.RFC3339), candle.Open, candle.High, candle.Low, candle.Close, candle.Volume)
	}
	return builder.String()
}

func withTimestamp(candles []market.Candle, index int, timestamp time.Time) []market.Candle {
	copy := append([]market.Candle(nil), candles...)
	copy[index].Timestamp = timestamp
	return copy
}
func withHigh(candles []market.Candle, index int, high float64) []market.Candle {
	copy := append([]market.Candle(nil), candles...)
	copy[index].High = high
	return copy
}
func swapped(candles []market.Candle, left, right int) []market.Candle {
	copy := append([]market.Candle(nil), candles...)
	copy[left], copy[right] = copy[right], copy[left]
	return copy
}
