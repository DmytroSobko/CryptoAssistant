package ath

import (
	"fmt"
	"sort"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
)

type dailyHigh struct {
	at   time.Time
	high float64
}

// AverageSpacedCandlePeaksBefore returns the mean of the highest count daily
// highs strictly before at, where each selected date is at least spacingMonths
// calendar months from every other selected date.
func AverageSpacedCandlePeaksBefore(candles []market.Candle, at time.Time, count, spacingMonths int) (float64, int, error) {
	if count < 1 {
		return 0, 0, fmt.Errorf("peak count must be at least 1")
	}
	if spacingMonths < 1 {
		return 0, 0, fmt.Errorf("peak spacing must be at least 1 month")
	}
	candidates := make([]dailyHigh, 0, len(candles))
	for _, candle := range candles {
		if candle.Timestamp.Before(at) && candle.High > 0 {
			candidates = append(candidates, dailyHigh{at: candle.Timestamp, high: candle.High})
		}
	}
	return averageSpacedCandidates(candidates, count, spacingMonths)
}

func averageSpacedCandidates(candidates []dailyHigh, count, spacingMonths int) (float64, int, error) {
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].high > candidates[j].high })
	selected := make([]dailyHigh, 0, count)
	for _, candidate := range candidates {
		separate := true
		for _, prior := range selected {
			if !monthsApart(candidate.at, prior.at, spacingMonths) {
				separate = false
				break
			}
		}
		if separate {
			selected = append(selected, candidate)
			if len(selected) == count {
				break
			}
		}
	}
	if len(selected) == 0 {
		return 0, 0, nil
	}
	var total float64
	for _, peak := range selected {
		total += peak.high
	}
	return total / float64(len(selected)), len(selected), nil
}

func monthsApart(a, b time.Time, months int) bool {
	if a.After(b) {
		a, b = b, a
	}
	return !b.Before(a.AddDate(0, months, 0))
}
