package ath

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type dailyHigh struct {
	at   time.Time
	high float64
}

// AverageSpacedPeaksBefore returns the mean of the highest count daily highs
// strictly before at, where each selected date is at least three calendar
// months from every other selected date. The CSV must use the app's canonical
// timestamp,open,high,low,close,volume format.
func AverageSpacedPeaksBefore(source string, at time.Time, count int) (float64, int, error) {
	if count < 1 {
		return 0, 0, fmt.Errorf("peak count must be at least 1")
	}
	path, err := resolveSource(source)
	if err != nil {
		return 0, 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return 0, 0, err
	}
	if len(rows) < 2 || len(rows[0]) < 3 || rows[0][0] != "timestamp" || rows[0][2] != "high" {
		return 0, 0, fmt.Errorf("historical peak source must be a daily OHLCV CSV")
	}
	candidates := make([]dailyHigh, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) < 3 {
			continue
		}
		day, parseErr := time.Parse(time.RFC3339, row[0])
		high, highErr := strconv.ParseFloat(row[2], 64)
		if parseErr == nil && highErr == nil && high > 0 && day.Before(at) {
			candidates = append(candidates, dailyHigh{day, high})
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].high > candidates[j].high })
	selected := make([]dailyHigh, 0, count)
	for _, candidate := range candidates {
		separate := true
		for _, prior := range selected {
			if !threeMonthsApart(candidate.at, prior.at) {
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

func threeMonthsApart(a, b time.Time) bool {
	if a.After(b) {
		a, b = b, a
	}
	return !b.Before(a.AddDate(0, 3, 0))
}

func resolveSource(source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return "", fmt.Errorf("historical peak source file is required")
	}
	for path := source; ; path = filepath.Join("..", path) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
		if strings.Count(path, ".."+string(filepath.Separator)) >= 3 {
			break
		}
	}
	return "", fmt.Errorf("historical peak source file %q was not found", source)
}
