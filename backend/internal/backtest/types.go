// Package backtest replays the pure strategy engine against completed daily
// candles. It owns no database, clock, HTTP, or live-portfolio dependency.
package backtest

import (
	"time"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

const ExecutionModelNextDailyOpen ExecutionModel = "NEXT_DAILY_OPEN"

type ExecutionModel string

type Request struct {
	Asset           string          `json:"asset"`
	Start           time.Time       `json:"start"`
	End             time.Time       `json:"end"`
	StartingCashUSD float64         `json:"startingCashUsd"`
	StrategyConfig  strategy.Config `json:"strategyConfig"`
	FeeBps          float64         `json:"feeBps"`
	SlippageBps     float64         `json:"slippageBps"`
	ExecutionModel  ExecutionModel  `json:"executionModel"`
}

type Assumptions struct {
	ExecutionModel               ExecutionModel `json:"executionModel"`
	SignalTiming                 string         `json:"signalTiming"`
	FillTiming                   string         `json:"fillTiming"`
	BuySizing                    string         `json:"buySizing"`
	SellSizing                   string         `json:"sellSizing"`
	FeeBps                       float64        `json:"feeBps"`
	SlippageBps                  float64        `json:"slippageBps"`
	NoMarginOrBorrowing          bool           `json:"noMarginOrBorrowing"`
	TaxesIncluded                bool           `json:"taxesIncluded"`
	ProfitTrigger2Implementation string         `json:"profitTrigger2Implementation"`
	ReferencePricePolicy         string         `json:"referencePricePolicy"`
	BreakEvenExitFloor           string         `json:"breakEvenExitFloor"`
}

type SimulationState struct {
	CashUSD       float64                 `json:"cashUsd"`
	Position      portfolio.Asset         `json:"position"`
	StrategyState strategy.PersistedState `json:"strategyState"`
	PendingOrder  *PendingOrder           `json:"pendingOrder,omitempty"`
}

type PendingOrder struct {
	SignalSequence         int                     `json:"signalSequence"`
	SignalTimestamp        time.Time               `json:"signalTimestamp"`
	ExecuteAt              time.Time               `json:"executeAt"`
	Action                 strategy.Action         `json:"action"`
	ActionPct              float64                 `json:"actionPct"`
	State                  strategy.State          `json:"state"`
	Reason                 string                  `json:"reason"`
	BreakEvenFloorNetPrice float64                 `json:"breakEvenFloorNetPrice,omitempty"`
	PriorStrategyState     strategy.PersistedState `json:"-"`
}

type Signal struct {
	Sequence               int             `json:"sequence"`
	Timestamp              time.Time       `json:"timestamp"`
	DecisionClose          float64         `json:"decisionClose"`
	Action                 strategy.Action `json:"action"`
	ActionPct              float64         `json:"actionPct"`
	StrategyState          strategy.State  `json:"strategyState"`
	Reason                 string          `json:"reason"`
	NextCondition          string          `json:"nextCondition"`
	OrderStatus            string          `json:"orderStatus"`
	BreakEvenFloorNetPrice float64         `json:"breakEvenFloorNetPrice,omitempty"`
}

type Trade struct {
	Sequence               int             `json:"sequence"`
	SignalSequence         int             `json:"signalSequence"`
	SignalTimestamp        time.Time       `json:"signalTimestamp"`
	ExecutionTimestamp     time.Time       `json:"executionTimestamp"`
	Side                   string          `json:"side"`
	Action                 strategy.Action `json:"action"`
	RequestedPct           float64         `json:"requestedPct"`
	Status                 string          `json:"status"`
	RawOpenPrice           float64         `json:"rawOpenPrice"`
	FillPrice              float64         `json:"fillPrice"`
	Quantity               float64         `json:"quantity"`
	GrossNotionalUSD       float64         `json:"grossNotionalUsd"`
	FeeUSD                 float64         `json:"feeUsd"`
	CashAfterUSD           float64         `json:"cashAfterUsd"`
	QuantityAfter          float64         `json:"quantityAfter"`
	AverageEntryAfter      float64         `json:"averageEntryAfter"`
	BreakEvenFloorNetPrice float64         `json:"breakEvenFloorNetPrice,omitempty"`
	Reason                 string          `json:"reason"`
}

type EquityPoint struct {
	Timestamp     time.Time `json:"timestamp"`
	ClosePrice    float64   `json:"closePrice"`
	CashUSD       float64   `json:"cashUsd"`
	Quantity      float64   `json:"quantity"`
	AssetValueUSD float64   `json:"assetValueUsd"`
	EquityUSD     float64   `json:"equityUsd"`
	PeakEquityUSD float64   `json:"peakEquityUsd"`
	DrawdownPct   float64   `json:"drawdownPct"`
}

type Summary struct {
	StartingCashUSD                 float64   `json:"startingCashUsd"`
	EndingCashUSD                   float64   `json:"endingCashUsd"`
	EndingAssetQuantity             float64   `json:"endingAssetQuantity"`
	EndingAssetValueUSD             float64   `json:"endingAssetValueUsd"`
	EndingEquityUSD                 float64   `json:"endingEquityUsd"`
	NetProfitLossUSD                float64   `json:"netProfitLossUsd"`
	ReturnPct                       float64   `json:"returnPct"`
	MaximumDrawdownPct              float64   `json:"maximumDrawdownPct"`
	FirstDataTimestamp              time.Time `json:"firstDataTimestamp"`
	LastDataTimestamp               time.Time `json:"lastDataTimestamp"`
	WarmupStart                     time.Time `json:"warmupStart"`
	SignalCount                     int       `json:"signalCount"`
	ExecutedTradeCount              int       `json:"executedTradeCount"`
	RejectedTradeCount              int       `json:"rejectedTradeCount"`
	UnexecutedTradeCount            int       `json:"unexecutedTradeCount"`
	TotalFeesUSD                    float64   `json:"totalFeesUsd"`
	BuyAndHoldEndingUSD             float64   `json:"buyAndHoldEndingUsd"`
	BuyAndHoldReturnPct             float64   `json:"buyAndHoldReturnPct"`
	BreakEvenFloorBlockedTradeCount int       `json:"breakEvenFloorBlockedTradeCount"`
}

type Result struct {
	Request         Request         `json:"request"`
	DataFingerprint string          `json:"dataFingerprint"`
	Assumptions     Assumptions     `json:"assumptions"`
	Summary         Summary         `json:"summary"`
	Signals         []Signal        `json:"signals"`
	Trades          []Trade         `json:"trades"`
	EquityCurve     []EquityPoint   `json:"equityCurve"`
	FinalState      SimulationState `json:"finalState"`
}
