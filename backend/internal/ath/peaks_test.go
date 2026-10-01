package ath

import (
	"math"
	"testing"
	"time"
)

func TestPeakBeforeUsesOnlyEarlierDailyHighs(t *testing.T) {
	firstDay := time.Date(2016, 9, 30, 0, 0, 0, 0, time.UTC)
	if _, found, err := PeakBefore("BTC", firstDay); err != nil || found {
		t.Fatalf("first source day must not be available to itself: found=%v err=%v", found, err)
	}
	peak, found, err := PeakBefore("BTC", firstDay.AddDate(0, 0, 1))
	if err != nil || !found || math.Abs(peak-608.99) > 1e-9 {
		t.Fatalf("peak before second source day = %v, %v, %v; want 608.99, true, nil", peak, found, err)
	}
}

func TestPeakBeforeRejectsUnsupportedAsset(t *testing.T) {
	if _, _, err := PeakBefore("SOL", time.Now().UTC()); err == nil {
		t.Fatal("expected unsupported asset error")
	}
}
