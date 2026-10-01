// Package ath provides the bundled, time-aware historical high reference used
// by the optional market-entry override. The values are derived from the
// tracked ten-year daily CSVs and are intentionally milestone-only: each point
// changes the running all-time high.
package ath

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

//go:embed btc-peaks.json
var btcPeaksJSON []byte

//go:embed eth-peaks.json
var ethPeaksJSON []byte

type point struct {
	At   time.Time `json:"at"`
	High float64   `json:"high"`
}

var (
	loadOnce sync.Once
	peaks    map[string][]point
	loadErr  error
)

func load() {
	peaks = make(map[string][]point)
	for asset, raw := range map[string][]byte{"BTC": btcPeaksJSON, "ETH": ethPeaksJSON} {
		var assetPeaks []point
		if err := json.Unmarshal(raw, &assetPeaks); err != nil {
			loadErr = fmt.Errorf("decode bundled %s historical peaks: %w", asset, err)
			return
		}
		peaks[asset] = assetPeaks
	}
}

// PeakBefore returns the highest bundled daily high known strictly before at.
// It never returns a same-day or future peak, so callers can use it in a
// backtest without leaking future prices into an earlier decision.
func PeakBefore(asset string, at time.Time) (float64, bool, error) {
	loadOnce.Do(load)
	if loadErr != nil {
		return 0, false, loadErr
	}
	assetPeaks, ok := peaks[asset]
	if !ok {
		return 0, false, fmt.Errorf("no bundled historical peaks for %s", asset)
	}
	var peak float64
	for _, candidate := range assetPeaks {
		if !candidate.At.Before(at) {
			break
		}
		peak = candidate.High
	}
	return peak, peak > 0, nil
}
