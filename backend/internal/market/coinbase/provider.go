// Package coinbase provides a read-only public market-data provider. It does
// not authenticate, access an account, or submit orders.
package coinbase

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

const (
	defaultBaseURL = "https://api.exchange.coinbase.com"
	maxDailyLimit  = 300
)

// Provider retrieves USD spot prices and daily OHLC data from Coinbase's
// public Exchange API. Its exported methods implement market.DataProvider.
type Provider struct {
	client  *http.Client
	baseURL string
	now     func() time.Time
}

var _ market.DataProvider = (*Provider)(nil)

func NewProvider(client *http.Client, baseURL string) *Provider {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultBaseURL
	}
	return &Provider{
		client:  client,
		baseURL: strings.TrimRight(baseURL, "/"),
		now:     time.Now,
	}
}

func (p *Provider) GetCurrentPrice(symbol string) (float64, error) {
	product, err := productFor(symbol)
	if err != nil {
		return 0, err
	}
	var response struct {
		Price string `json:"price"`
	}
	if err := p.getJSON("/products/"+product+"/ticker", nil, &response); err != nil {
		return 0, fmt.Errorf("get %s current price: %w", symbol, err)
	}
	price, err := strconv.ParseFloat(response.Price, 64)
	if err != nil || !validPositive(price) {
		return 0, fmt.Errorf("invalid %s current price %q", symbol, response.Price)
	}
	return price, nil
}

// GetDailyCandles requests calendar-day candles and removes today's unclosed
// candle. Returned data is chronological and contains only completed UTC days.
func (p *Provider) GetDailyCandles(symbol string, limit int) ([]market.Candle, error) {
	product, err := productFor(symbol)
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > maxDailyLimit {
		return nil, fmt.Errorf("daily candle limit must be between 1 and %d", maxDailyLimit)
	}

	now := p.now().UTC()
	dayStart := now.Truncate(24 * time.Hour)
	query := url.Values{
		"granularity": {"86400"},
		"start":       {dayStart.AddDate(0, 0, -limit).Format(time.RFC3339)},
		"end":         {now.Format(time.RFC3339)},
	}
	var rows [][]json.RawMessage
	if err := p.getJSON("/products/"+product+"/candles", query, &rows); err != nil {
		return nil, fmt.Errorf("get %s daily candles: %w", symbol, err)
	}

	candles := make([]market.Candle, 0, len(rows))
	for _, row := range rows {
		candle, err := parseCandle(row)
		if err != nil {
			return nil, fmt.Errorf("decode %s daily candle: %w", symbol, err)
		}
		if !candle.Timestamp.Before(dayStart) {
			continue
		}
		candles = append(candles, candle)
	}
	sort.Slice(candles, func(i, j int) bool { return candles[i].Timestamp.Before(candles[j].Timestamp) })
	if len(candles) > limit {
		candles = candles[len(candles)-limit:]
	}
	if len(candles) == 0 {
		return nil, fmt.Errorf("provider returned no completed daily candles")
	}
	return candles, nil
}

func (p *Provider) getJSON(path string, query url.Values, target any) error {
	requestURL := p.baseURL + path
	if query != nil {
		requestURL += "?" + query.Encode()
	}
	req, err := http.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	response, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
		return fmt.Errorf("unexpected HTTP status %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func productFor(symbol string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(symbol)) {
	case "BTC":
		return "BTC-USD", nil
	case "ETH":
		return "ETH-USD", nil
	default:
		return "", fmt.Errorf("unsupported asset %q: expected BTC or ETH", symbol)
	}
}

func parseCandle(row []json.RawMessage) (market.Candle, error) {
	if len(row) != 6 {
		return market.Candle{}, fmt.Errorf("expected 6 fields, got %d", len(row))
	}
	values := make([]float64, len(row))
	for i, raw := range row {
		value, err := parseNumber(raw)
		if err != nil {
			return market.Candle{}, fmt.Errorf("field %d: %w", i, err)
		}
		values[i] = value
	}
	candle := market.Candle{
		Timestamp: time.Unix(int64(values[0]), 0).UTC(),
		Low:       values[1],
		High:      values[2],
		Open:      values[3],
		Close:     values[4],
		Volume:    values[5],
	}
	if candle.Timestamp.IsZero() || !validPositive(candle.Open) || !validPositive(candle.High) || !validPositive(candle.Low) || !validPositive(candle.Close) || candle.Low > candle.High || candle.Open < candle.Low || candle.Open > candle.High || candle.Close < candle.Low || candle.Close > candle.High || math.IsNaN(candle.Volume) || math.IsInf(candle.Volume, 0) || candle.Volume < 0 {
		return market.Candle{}, fmt.Errorf("invalid OHLC values")
	}
	return candle, nil
}

func parseNumber(raw json.RawMessage) (float64, error) {
	var stringValue string
	if err := json.Unmarshal(raw, &stringValue); err == nil {
		return strconv.ParseFloat(stringValue, 64)
	}
	var numberValue float64
	if err := json.Unmarshal(raw, &numberValue); err != nil {
		return 0, err
	}
	return numberValue, nil
}

func validPositive(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}
