package backtest

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

func TestRunIsDeterministicAndDoesNotMutateCandles(t *testing.T) {
	candles := entryFixture()
	original := append([]market.Candle(nil), candles...)
	request := fixtureRequest(candles, 199, len(candles)-1)

	first, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same input produced different results\nfirst=%+v\nsecond=%+v", first, second)
	}
	if !reflect.DeepEqual(candles, original) {
		t.Fatal("Run mutated its candle input")
	}
	if len(first.Signals) != len(candles)-199 {
		t.Fatalf("signals = %d, want %d", len(first.Signals), len(candles)-199)
	}
}

func TestRunFillsAtNextDailyOpenWithoutLookahead(t *testing.T) {
	candles := entryFixture()
	request := fixtureRequest(candles, 199, len(candles)-1)
	result, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}

	var buySignal Signal
	for _, signal := range result.Signals {
		if signal.Action == strategy.ActionBuy40 {
			buySignal = signal
			break
		}
	}
	if buySignal.Sequence == 0 {
		t.Fatalf("expected a BUY_40 signal, got %+v", result.Signals)
	}
	if len(result.Trades) != 1 {
		t.Fatalf("trades = %+v, want one buy", result.Trades)
	}
	trade := result.Trades[0]
	if !trade.ExecutionTimestamp.Equal(buySignal.Timestamp.AddDate(0, 0, 1)) {
		t.Fatalf("fill timestamp %s, signal timestamp %s; expected next daily open", trade.ExecutionTimestamp, buySignal.Timestamp)
	}
	if trade.RawOpenPrice != 96 {
		t.Fatalf("raw open = %v, want 96", trade.RawOpenPrice)
	}
	if math.Abs(trade.FillPrice-96.048) > 1e-9 {
		t.Fatalf("fill price = %.12f, want 96.048", trade.FillPrice)
	}
	if trade.Status != "EXECUTED" || buySignal.OrderStatus != "EXECUTED" {
		t.Fatalf("trade/signal not marked executed: %+v / %+v", trade, buySignal)
	}

	withFuture := append(append([]market.Candle(nil), candles...), candleAt(len(candles), 200))
	changedFuture := append([]market.Candle(nil), withFuture...)
	changedFuture[len(changedFuture)-1] = candleAt(len(changedFuture)-1, 2)
	longRequest := fixtureRequest(withFuture, 199, len(withFuture)-1)
	baseline, err := Run(longRequest, withFuture)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := Run(longRequest, changedFuture)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(baseline.Trades[0], changed.Trades[0]) {
		t.Fatalf("a future candle changed an earlier trade\nbaseline=%+v\nchanged=%+v", baseline.Trades[0], changed.Trades[0])
	}
}

func TestRunATHOverrideDoesNotUseSameDayOrFutureHigh(t *testing.T) {
	start := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	candles := make([]market.Candle, 0, 204)
	for index := 0; index < 200; index++ {
		candles = append(candles, candleAtDate(start.AddDate(0, 0, index), 100))
	}
	// This completed close is below 60% of the known 101 high. Its own high
	// is deliberately much larger and must not be used for this decision.
	decision := candleAtDate(start.AddDate(0, 0, 200), 60)
	decision.High = 1_000
	candles = append(candles, decision, candleAtDate(start.AddDate(0, 0, 201), 60), candleAtDate(start.AddDate(0, 0, 202), 60), candleAtDate(start.AddDate(0, 0, 203), 60))
	request := fixtureRequest(candles, 199, len(candles)-1)
	request.StrategyConfig.ATHEntryOverrideEnabled = true
	request.StrategyConfig.ATHEntryThresholdPct = 60

	result, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}
	var signal Signal
	for _, candidate := range result.Signals {
		if candidate.Timestamp.Equal(decision.Timestamp) {
			signal = candidate
			break
		}
	}
	if signal.Action != strategy.ActionBuy40 || signal.DecisionClose != 60 {
		t.Fatalf("ATH override used same-day high or did not enter: %+v", signal)
	}
}

func TestRunRecordsEndOfDataActionWithoutAFill(t *testing.T) {
	candles := entryFixture()
	request := fixtureRequest(candles, 199, len(candles)-3) // The breakout candle is now the final decision candle.
	result, err := Run(request, candles)
	if err != nil {
		t.Fatal(err)
	}
	last := result.Signals[len(result.Signals)-1]
	if last.Action != strategy.ActionBuy40 || last.OrderStatus != "UNEXECUTED_END_OF_DATA" {
		t.Fatalf("last signal = %+v", last)
	}
	if len(result.Trades) != 0 || result.Summary.UnexecutedTradeCount != 1 {
		t.Fatalf("end-of-data signal must not create a fill: trades=%+v summary=%+v", result.Trades, result.Summary)
	}
}

func TestRunRejectsInsufficientWarmupAndInvalidCandle(t *testing.T) {
	candles := entryFixture()
	request := fixtureRequest(candles, 198, len(candles)-1)
	if _, err := Run(request, candles); err == nil {
		t.Fatal("expected insufficient warm-up error")
	}
	request = fixtureRequest(candles, 199, len(candles)-1)
	invalid := append([]market.Candle(nil), candles...)
	invalid[200].High = invalid[200].Low - 1
	if _, err := Run(request, invalid); err == nil {
		t.Fatal("expected invalid OHLC error")
	}
}

func entryFixture() []market.Candle {
	candles := make([]market.Candle, 0, 209)
	for index := 0; index < 200; index++ {
		candles = append(candles, candleAt(index, 100))
	}
	for _, price := range []float64{100, 120, 100, 80, 90, 85, 95, 96} {
		candles = append(candles, candleAt(len(candles), price))
	}
	// The open is deliberately different from the prior signal close.
	fill := candleAt(len(candles), 110)
	fill.Open, fill.High, fill.Low, fill.Close = 110, 111, 109, 110
	return append(candles, fill)
}

func fixtureRequest(candles []market.Candle, start, end int) Request {
	return Request{Asset: "BTC", Start: candles[start].Timestamp, End: candles[end].Timestamp, StartingCashUSD: 10000, StrategyConfig: testConfig(), FeeBps: 10, SlippageBps: 5, ExecutionModel: ExecutionModelNextDailyOpen}
}

func testConfig() strategy.Config {
	return strategy.Config{PullbackMinPct: 10, PivotLeft: 1, PivotRight: 1, TrendMode: strategy.TrendModeOff, Entry1Pct: 40, Entry2Pct: 30, Entry3Pct: 30, ProfitTrigger1Pct: 10, ProfitTrigger2Pct: 20, ProfitTakePct: 25, Drawdown1Pct: -10, Drawdown1SellPct: 20, Drawdown2Pct: -15, Drawdown2SellPct: 30, Drawdown3Pct: -20, Drawdown3SellPct: 70}
}

func candleAt(index int, price float64) market.Candle {
	stamp := time.Date(2020, 1, 1+index, 0, 0, 0, 0, time.UTC)
	return candleAtDate(stamp, price)
}

func candleAtDate(stamp time.Time, price float64) market.Candle {
	return market.Candle{Timestamp: stamp, Open: price, High: price + 1, Low: price - 1, Close: price, Volume: 1}
}
