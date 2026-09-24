// Package strategy will contain the pure deterministic strategy engine. It
// deliberately imports neither HTTP, SQLite, market providers, nor UI code.
package strategy

import (
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/portfolio"
)

type Action string

const (
	ActionHold           Action = "HOLD"
	ActionWait           Action = "WAIT"
	ActionBuy40          Action = "BUY_40"
	ActionBuy30          Action = "BUY_30"
	ActionBuy30Final     Action = "BUY_30_FINAL"
	ActionSellProfit     Action = "SELL_PROFIT"
	ActionSellDrawdown10 Action = "SELL_DRAWDOWN_10"
	ActionSellDrawdown15 Action = "SELL_DRAWDOWN_15"
	ActionSellDrawdown20 Action = "SELL_DRAWDOWN_20"
)

type State string

const (
	StateCash              State = "CASH"
	StateCorrection        State = "CORRECTION"
	StateRecoveryPending   State = "RECOVERY_PENDING"
	StateEntryPending      State = "ENTRY_PENDING"
	StatePartialPosition   State = "PARTIAL_POSITION"
	StateFullPosition      State = "FULL_POSITION"
	StateProfitProtection  State = "PROFIT_PROTECTION"
	StateExiting           State = "EXITING"
	StateWaitingForReentry State = "WAITING_FOR_REENTRY"
)

type Config struct {
	PullbackMinPct    float64 `json:"pullbackMinPct"`
	PivotLeft         int     `json:"pivotLeft"`
	PivotRight        int     `json:"pivotRight"`
	BreakoutBufferPct float64 `json:"breakoutBufferPct"`
	TrendMode         string  `json:"trendMode"`
	Entry1Pct         float64 `json:"entry1Pct"`
	Entry2Pct         float64 `json:"entry2Pct"`
	Entry3Pct         float64 `json:"entry3Pct"`
	ProfitTrigger1Pct float64 `json:"profitTrigger1Pct"`
	ProfitTrigger2Pct float64 `json:"profitTrigger2Pct"`
	ProfitTakePct     float64 `json:"profitTakePct"`
	Drawdown1Pct      float64 `json:"drawdown1Pct"`
	Drawdown1SellPct  float64 `json:"drawdown1SellPct"`
	Drawdown2Pct      float64 `json:"drawdown2Pct"`
	Drawdown2SellPct  float64 `json:"drawdown2SellPct"`
	Drawdown3Pct      float64 `json:"drawdown3Pct"`
	Drawdown3SellPct  float64 `json:"drawdown3SellPct"`
}

type PersistedState struct {
	Asset        string  `json:"asset"`
	CurrentState State   `json:"state"`
	LocalHigh    float64 `json:"localHigh"`
	LocalLow     float64 `json:"localLow"`
	HigherLow    float64 `json:"higherLow"`
	HighestPrice float64 `json:"highestPrice"`
	DrawdownPct  float64 `json:"drawdownPct"`
}

type Result struct {
	Asset               string  `json:"asset"`
	Action              Action  `json:"action"`
	State               State   `json:"state"`
	Price               float64 `json:"price"`
	Trend               string  `json:"trend"`
	PositionPct         float64 `json:"positionPct"`
	PullbackPct         float64 `json:"pullbackPct"`
	LocalHigh           float64 `json:"localHigh"`
	LocalLow            float64 `json:"localLow"`
	DrawdownFromHighPct float64 `json:"drawdownFromHighPct"`
	Reason              string  `json:"reason"`
	NextCondition       string  `json:"nextCondition"`
}

// Engine is intentionally only a seam at this stage. The next implementation
// phase will make Evaluate a pure function of candles, position, config, and
// persisted state.
type Engine interface {
	Evaluate(candles []market.Candle, position portfolio.Asset, config Config, state PersistedState) (Result, PersistedState)
}
