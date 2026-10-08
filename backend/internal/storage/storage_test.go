package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func TestMigrateAndPersistPortfolio(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	input := portfolio.Portfolio{
		CashBalance: 2500,
		Assets:      []portfolio.Asset{{Symbol: "BTC", Quantity: 0.12, AverageEntryPrice: 71200, CashAllocated: 8544}},
	}
	if err := store.SavePortfolio(context.Background(), input); err != nil {
		t.Fatalf("save portfolio: %v", err)
	}
	actual, err := store.GetPortfolio(context.Background())
	if err != nil {
		t.Fatalf("get portfolio: %v", err)
	}
	if actual.CashBalance != 2500 || len(actual.Assets) != 2 {
		t.Fatalf("unexpected portfolio: %+v", actual)
	}
	if actual.Assets[0].Symbol != "BTC" || actual.Assets[0].Quantity != 0.12 {
		t.Fatalf("BTC was not persisted: %+v", actual.Assets[0])
	}
}

func TestMigrateIsIdempotentAndConfigDefaultsAreIndependent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("initial migrate: %v", err)
	}
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("repeat migrate: %v", err)
	}

	btc, err := store.GetStrategyConfig(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get BTC config: %v", err)
	}
	eth, err := store.GetStrategyConfig(context.Background(), "ETH")
	if err != nil {
		t.Fatalf("get ETH config: %v", err)
	}
	if btc.PullbackMinPct != 15 || eth.PullbackMinPct != 35 || btc.RecoveryEntryMinPeakDiscountPct != 10 || eth.RecoveryEntryMinPeakDiscountPct != 0 {
		t.Fatalf("unexpected independent defaults: BTC=%+v ETH=%+v", btc, eth)
	}
	for _, asset := range []string{"BTC", "ETH"} {
		profile, err := store.GetAssetStrategySettings(context.Background(), asset)
		if err != nil {
			t.Fatal(err)
		}
		want, err := DefaultAssetStrategySettings(asset).Normalized()
		if err != nil || profile != want {
			t.Fatalf("%s migrated defaults differ from constructor: got=%+v want=%+v err=%v", asset, profile, want, err)
		}
	}
}

func TestPersistMarketCandlesAndCurrentPriceSeparately(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	dayOne := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	candles := []market.Candle{
		{Timestamp: dayOne, Open: 90, High: 110, Low: 80, Close: 100, Volume: 42},
		{Timestamp: dayOne.AddDate(0, 0, 1), Open: 100, High: 120, Low: 95, Close: 115, Volume: 64},
	}
	if err := store.SaveDailyCandles(context.Background(), "btc", candles); err != nil {
		t.Fatalf("save candles: %v", err)
	}
	if err := store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "BTC", Price: 117, UpdatedAt: dayOne.AddDate(0, 0, 2).Add(3 * time.Hour)}); err != nil {
		t.Fatalf("save current price: %v", err)
	}

	actualCandles, err := store.ListDailyCandles(context.Background(), "BTC", 200)
	if err != nil {
		t.Fatalf("list candles: %v", err)
	}
	if len(actualCandles) != 2 || actualCandles[0].Close != 100 || actualCandles[1].Close != 115 {
		t.Fatalf("unexpected candles: %+v", actualCandles)
	}
	snapshot, err := store.LatestSnapshot(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get current price: %v", err)
	}
	if snapshot.Price != 117 || !snapshot.UpdatedAt.Equal(dayOne.AddDate(0, 0, 2).Add(3*time.Hour)) {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}

	if err := store.SaveDailyCandles(context.Background(), "BTC", []market.Candle{{Timestamp: dayOne, Open: 91, High: 111, Low: 81, Close: 101, Volume: 43}}); err != nil {
		t.Fatalf("upsert candle: %v", err)
	}
	actualCandles, err = store.ListDailyCandles(context.Background(), "BTC", 0)
	if err != nil || actualCandles[0].Close != 101 {
		t.Fatalf("candle was not updated: candles=%+v err=%v", actualCandles, err)
	}
}

func TestMarketStorageRejectsInvalidValues(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	err = store.SaveCurrentPrice(context.Background(), market.Snapshot{Symbol: "DOGE", Price: 1, UpdatedAt: time.Now()})
	if err == nil {
		t.Fatalf("expected invalid asset error, got %v", err)
	}
}

func TestPersistExpandedStrategyStateAndEvents(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	initial, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get initial state: %v", err)
	}
	if initial.Asset != "BTC" || initial.CurrentState != strategy.StateCash {
		t.Fatalf("unexpected initial state: %+v", initial)
	}

	anchor := time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC)
	expected := strategy.PersistedState{
		Asset: "BTC", StrategyID: strategy.StrategyRecoveryBreakout, CurrentState: strategy.StateProfitProtection,
		LocalHigh: 120, LocalLow: 90, HigherLow: 100, HighestPrice: 140, DrawdownPct: -8,
		CorrectionHighAt: anchor, LastBreakoutHighAt: anchor.AddDate(0, 0, 5), LastBreakoutHigherLowAt: anchor.AddDate(0, 0, 4), ReentryAfter: anchor.AddDate(0, 0, -3),
		EntryStep: 2, ProfitTaken: true, Drawdown1Triggered: true, Drawdown2Triggered: true, PositionOpen: true,
	}
	if err := store.SaveStrategyState(context.Background(), expected); err != nil {
		t.Fatalf("save state: %v", err)
	}
	actual, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil {
		t.Fatalf("get state: %v", err)
	}
	if actual != expected {
		t.Fatalf("state round trip mismatch: got %+v, want %+v", actual, expected)
	}

	event := StrategyEvent{Asset: "BTC", Timestamp: anchor.Add(12 * time.Hour), Action: string(strategy.ActionSellProfit), Price: 135, Reason: "Configured profit target reached.", State: string(strategy.StateProfitProtection)}
	if err := store.AppendStrategyEvent(context.Background(), event); err != nil {
		t.Fatalf("append event: %v", err)
	}
	events, err := store.ListStrategyEvents(context.Background(), 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 1 || events[0].Asset != event.Asset || events[0].Action != event.Action || events[0].Price != event.Price || !events[0].Timestamp.Equal(event.Timestamp) {
		t.Fatalf("event round trip mismatch: got %+v, want %+v", events, event)
	}
}

func TestPerAssetStrategyProfilesKeepConfigurationsAndStatesIndependent(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatal(err)
	}

	profile, err := store.GetAssetStrategySettings(context.Background(), "BTC")
	if err != nil {
		t.Fatal(err)
	}
	if profile.SelectedStrategyID != strategy.StrategyRecoveryBreakout || profile.RecoveryBreakout.StrategyID != strategy.StrategyRecoveryBreakout || profile.DipAccumulation.StrategyID != strategy.StrategyDipAccumulation {
		t.Fatalf("unexpected migrated BTC profile: %+v", profile)
	}
	if !profile.ATHEntryOverride.Enabled || profile.ATHEntryOverride.ThresholdPct != 60 {
		t.Fatalf("unexpected default BTC ATH entry override: %+v", profile.ATHEntryOverride)
	}

	recoveryState := strategy.PersistedState{Asset: "BTC", StrategyID: strategy.StrategyRecoveryBreakout, CurrentState: strategy.StatePartialPosition, EntryStep: 1}
	if err := store.SaveStrategyState(context.Background(), recoveryState); err != nil {
		t.Fatal(err)
	}
	profile.SelectedStrategyID = strategy.StrategyDipAccumulation
	profile.DipAccumulation.Entry1Pct = 35
	profile.DipAccumulation.Entry3Pct = 35
	profile.DipAccumulation.RecoveryEntryMinPeakDiscountPct = 20
	profile.ATHEntryOverride = strategy.ATHEntryOverrideSettings{Enabled: true, ThresholdPct: 55}
	if err := store.SaveAssetStrategySettings(context.Background(), "BTC", profile); err != nil {
		t.Fatal(err)
	}
	active, err := store.GetStrategyConfig(context.Background(), "BTC")
	if err != nil || active.StrategyID != strategy.StrategyDipAccumulation || active.Entry1Pct != 35 {
		t.Fatalf("active Dip Accumulation config=%+v err=%v", active, err)
	}
	storedProfile, err := store.GetAssetStrategySettings(context.Background(), "BTC")
	if err != nil || !storedProfile.ATHEntryOverride.Enabled || storedProfile.ATHEntryOverride.ThresholdPct != 55 || storedProfile.DipAccumulation.RecoveryEntryMinPeakDiscountPct != 20 {
		t.Fatalf("stored BTC ATH entry override=%+v err=%v", storedProfile.ATHEntryOverride, err)
	}
	ethProfile, err := store.GetAssetStrategySettings(context.Background(), "ETH")
	if err != nil || !ethProfile.ATHEntryOverride.Enabled || ethProfile.ATHEntryOverride.ThresholdPct != 60 {
		t.Fatalf("ETH override was changed with BTC: %+v err=%v", ethProfile.ATHEntryOverride, err)
	}
	dipState, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil || dipState.StrategyID != strategy.StrategyDipAccumulation || dipState.EntryStep != 0 {
		t.Fatalf("newly selected Dip Accumulation state=%+v err=%v; want fresh state", dipState, err)
	}
	dipState.CurrentState = strategy.StatePartialPosition
	dipState.EntryStep = 1
	dipState.FirstEntryReferencePrice = 100
	if err := store.SaveStrategyState(context.Background(), dipState); err != nil {
		t.Fatal(err)
	}

	profile.SelectedStrategyID = strategy.StrategyRecoveryBreakout
	if err := store.SaveAssetStrategySettings(context.Background(), "BTC", profile); err != nil {
		t.Fatal(err)
	}
	recoveryAfterSwitch, err := store.GetStrategyState(context.Background(), "BTC")
	if err != nil || recoveryAfterSwitch.StrategyID != strategy.StrategyRecoveryBreakout || recoveryAfterSwitch.EntryStep != 0 {
		t.Fatalf("reselected Recovery Breakout state=%+v err=%v; want a fresh state", recoveryAfterSwitch, err)
	}
	storedDip, err := getStrategyStateForStrategy(context.Background(), store.db, "BTC", strategy.StrategyDipAccumulation)
	if err != nil || storedDip.EntryStep != 1 || storedDip.FirstEntryReferencePrice != 100 {
		t.Fatalf("switching away corrupted Dip Accumulation state=%+v err=%v", storedDip, err)
	}
}

func TestStrategyStorageRejectsInvalidState(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "assistant.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()
	if err := store.Migrate("../../migrations"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := store.SaveStrategyState(context.Background(), strategy.PersistedState{Asset: "BTC", CurrentState: "UNKNOWN"}); err == nil {
		t.Fatal("expected invalid state to be rejected")
	}
	if err := store.SaveStrategyConfig(context.Background(), "BTC", strategy.Config{}); err == nil {
		t.Fatal("expected invalid strategy configuration to be rejected")
	}
	if err := store.AppendStrategyEvent(context.Background(), StrategyEvent{Asset: "DOGE", Timestamp: time.Now(), Action: "HOLD", Price: 1, Reason: "test", State: "CASH"}); err == nil {
		t.Fatal("expected invalid event asset to be rejected")
	}
}
