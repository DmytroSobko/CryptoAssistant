# Crypto Strategy Assistant — Local MVP Specification

## 1. Purpose

Build a local desktop decision-support application for BTC and ETH.

The app reads market data, tracks the user's manually entered portfolio, evaluates a deterministic trading strategy, and tells the user what the strategy says to do **right now**.

The application is an advisory tool, not an autonomous trading bot.

### MVP explicitly does NOT include

- Backtesting
- Paper trading
- AI-generated trading signals
- Autonomous order execution
- Binance integration
- Wealthsimple integration

These can be added later without rewriting the core strategy engine.

---

# 2. Core User Experience

The user opens the application and immediately sees:

```text
BTC
Price:              $84,600
Trend:              BULLISH
Position:           0.12 BTC
Average Entry:      $71,200
P/L:                +18.5%
Local High:         $87,400
Drawdown:           -3.2%

ACTION: HOLD

Reason:
Profit is +18.5%, but price is only 3.2% below
the local high and no drawdown exit has triggered.
```

For a new entry:

```text
BTC
Price:              $84,600

ACTION: BUY 40%

Reason:
Correction exceeded 10%, a Higher Low formed,
and price broke the previous local high.
Trend filter is bullish.
```

The user should always be able to answer:

1. What is the current market state?
2. What does the strategy recommend?
3. Why?
4. What conditions would cause the next action?
5. What is my current position?

---

# 3. Technology Stack

## Desktop

- Tauri
- React
- TypeScript
- Vite

## Backend

- Go
- REST API
- Deterministic strategy engine

## Database

- SQLite

## Development environment

- macOS
- VS Code
- Git

The application should be easy to run locally with one documented development command.

---

# 4. Architecture

```text
React / TypeScript UI
        |
        | HTTP
        v
Go API
        |
        +------------------+
        |                  |
        v                  v
Market Data          Portfolio Store
        |                  |
        +--------+---------+
                 |
                 v
          Strategy Engine
                 |
                 v
            Signal Result
                 |
                 v
              SQLite
```

Important architectural rule:

**The strategy engine must not depend on React, Tauri, Binance, Wealthsimple, or AI.**

It should accept market/portfolio data as inputs and return a deterministic decision.

This makes the strategy independently testable and allows future integrations.

---

# 5. MVP Scope

## Required

### Market data

Initially use a public market-data source or manually supplied data.

The application needs:

- current BTC price
- current ETH price
- daily OHLC candles
- enough historical daily candles to calculate indicators
- timestamp of the latest data

### Portfolio

User can manually enter:

- BTC quantity
- BTC average entry price
- ETH quantity
- ETH average entry price
- cash balance
- optional transaction history

### Strategy engine

Evaluate:

- correction
- local lows/highs
- Higher Low
- breakout
- trend
- profit level
- high-water mark
- drawdown from high
- current strategy state

### Dashboard

Show:

- BTC
- ETH
- current price
- trend
- position
- average entry
- P/L
- local high
- drawdown
- current action
- reason
- next trigger

### Alerts

Allow local notifications for important strategy changes.

Examples:

- BUY 40%
- BUY 30%
- BUY 30%
- SELL 25%
- SELL 20%
- SELL 30%
- SELL 70%
- trend changed
- new breakout
- new drawdown threshold

---

# 6. Strategy Philosophy

The strategy intentionally does NOT try to:

- buy the exact bottom
- sell the exact top
- predict the future
- react to every small price movement

Instead:

**Buy after recovery is confirmed.**

**Reduce exposure after meaningful deterioration.**

The strategy should prioritize price structure over prediction.

---

# 7. Timeframe

Primary timeframe:

**1D / daily candles**

The MVP should make decisions using completed daily candles.

Intraday prices can be displayed, but the core signal should not change repeatedly during the day.

This prevents excessive signal noise.

---

# 8. Strategy State Machine

The strategy should have explicit states.

```text
CASH
  |
  v
CORRECTION
  |
  v
RECOVERY_PENDING
  |
  v
ENTRY_PENDING
  |
  v
PARTIAL_POSITION
  |
  v
FULL_POSITION
  |
  v
PROFIT_PROTECTION
  |
  v
EXITING
  |
  v
WAITING_FOR_REENTRY
  |
  +------> RECOVERY_PENDING
```

The engine should always know the current state.

---

# 9. Correction Detection

A correction becomes eligible when:

```text
Current Price <= Recent Local High × (1 - pullback_min_pct)
```

Default:

```text
pullback_min_pct = 10%
```

Example:

```text
Local High = $100,000
10% correction threshold = $90,000
```

Price reaching $90,000 does NOT automatically generate a BUY signal.

It only makes the market eligible for recovery analysis.

---

# 10. Recovery Confirmation

The strategy waits for evidence that the decline has stabilized.

Basic structure:

```text
High
  \
   \
    Low
      \
       Higher Low
            \
             Breakout
```

Required elements:

1. Meaningful correction occurred.
2. A local low formed.
3. Price recovered.
4. A Higher Low formed.
5. Price broke above the relevant previous local high.

Example:

```text
100k
  |
  |
  80k  <- correction / local low
   \
    85k <- recovery high
     \
      82k <- Higher Low
        \
         90k <- breakout
```

The breakout is the confirmation event.

Do NOT buy simply because price has fallen 10–20%.

---

# 11. Local High / Low Detection

The MVP should use a simple deterministic method.

Suggested default:

```text
pivot_left = 2
pivot_right = 2
```

A local high is a candle whose high is greater than the highs of the surrounding pivot window.

A local low is the inverse.

These values must be configurable.

Avoid complicated machine-learning pattern detection.

---

# 12. Breakout Rule

A recovery breakout occurs when:

```text
Daily Close > Previous Relevant Local High
```

Prefer a confirmed daily close rather than an intraday price spike.

Optional configurable breakout buffer:

```text
breakout_buffer_pct = 0%
```

Future example:

```text
breakout_buffer_pct = 0.5%
```

---

# 13. Trend Filter

The MVP should support configurable trend modes.

## STRICT

Entry requires:

```text
Close > SMA200
```

## RECOVERY

Entry may occur below SMA200 if the recovery structure is strong.

## OFF

No moving-average filter.

Default:

```text
STRICT
```

Indicators:

- SMA50
- SMA200

Optional additional condition:

```text
SMA50 > SMA50_previous
```

meaning the 50-day moving average is rising.

The trend filter is a filter, not a prediction.

---

# 14. Entry Strategy

Once a valid recovery breakout is confirmed:

### Entry 1

Buy:

**40%**

### Entry 2

Buy:

**30%**

### Entry 3

Buy:

**30%**

The exact mechanics should be configurable.

Suggested implementation:

```text
entry_1_pct = 40%
entry_2_pct = 30%
entry_3_pct = 30%
```

The app should clearly display:

```text
BUY 40%
```

rather than simply:

```text
BUY
```

---

# 15. Entry State

The engine must remember which entry steps have already occurred.

Example:

```text
Entry:
40% completed
30% pending
30% pending
```

After the second entry:

```text
40% completed
30% completed
30% pending
```

This prevents duplicate signals.

---

# 16. Profit Protection

Profit is calculated relative to weighted average entry price.

```text
P/L % =
(Current Price - Average Entry) / Average Entry
```

Example:

```text
Average Entry = $70,000
Current Price = $84,000

P/L = +20%
```

---

# 17. Profit-Taking Rule

Default profit zone:

```text
+10% to +20%
```

When the position reaches the configured profit trigger, sell:

**25% of the current position.**

Important:

This means 25% of the CURRENT POSITION.

It does NOT mean 25% of the profit.

Example:

```text
Position value = $12,000

Sell 25% = $3,000

Remaining BTC position = $9,000
Cash = $3,000
```

Default:

```text
profit_take_pct = 25%
profit_trigger_1 = 10%
profit_trigger_2 = 20%
```

For the MVP, implement a simple one-time profit-zone action rather than repeatedly selling on every price tick.

The exact behavior should be explicit in configuration.

---

# 18. High-Water Mark

While holding BTC/ETH, track:

```text
highest_price_since_entry
```

Example:

```text
Entry = $70k
Price rises:
72k
75k
80k
85k
90k
```

High-water mark:

```text
$90k
```

Drawdown is:

```text
(Current Price - High Water Mark) / High Water Mark
```

Example:

```text
High = $90k
Current = $81k

Drawdown = -10%
```

---

# 19. Drawdown Exit Rules

Default:

### -10%

Sell:

**20% of current position**

### -15%

Sell:

**30% of current position**

### -20%

Sell:

**70% of current position**

Configuration:

```text
drawdown_1 = -10%
drawdown_1_sell = 20%

drawdown_2 = -15%
drawdown_2_sell = 30%

drawdown_3 = -20%
drawdown_3_sell = 70%
```

The system should execute each threshold only once per high-water-mark cycle.

Example:

```text
High = $100k

90k -> SELL 20%
85k -> SELL 30%
80k -> SELL 70%
```

Do not repeatedly trigger the same action while price remains below the threshold.

---

# 20. New High Reset

If BTC/ETH establishes a new high:

```text
highest_price_since_entry = new high
```

The drawdown thresholds become eligible again relative to the new high.

Example:

```text
High = 100k
Price drops to 92k -> -8%, no action

Price rises to 105k
New high = 105k

Price falls to 94k
Drawdown = -10.5%

SELL 20%
```

---

# 21. Re-entry

After a large exit, the system must NOT immediately buy back simply because price has fallen.

For example:

```text
100k
80k
```

does not create a BUY signal.

The system must wait for:

1. Correction/stabilization
2. Local low
3. Recovery
4. Higher Low
5. Break above relevant local high
6. Trend filter, if enabled

Then:

```text
BUY 40%
BUY 30%
BUY 30%
```

This is a key rule.

---

# 22. BTC and ETH

Use the same strategy framework for both assets.

However, all parameters must be configurable independently.

Example:

```yaml
BTC:
  pullback_min_pct: 10
  profit_trigger_pct: 10
  drawdown_1: 10
  drawdown_2: 15
  drawdown_3: 20

ETH:
  pullback_min_pct: 15
  profit_trigger_pct: 15
  drawdown_1: 12
  drawdown_2: 18
  drawdown_3: 25
```

These ETH values are starting configuration examples only.

They are NOT claimed to be optimal.

Do not hard-code BTC parameters into ETH logic.

---

# 23. Optional BTC Regime Filter for ETH

Later, ETH can optionally use BTC as a market-regime filter.

Example:

```text
BTC trend = bearish
ETH BUY signal = blocked or reduced
```

For MVP this should be optional and preferably disabled until explicitly enabled.

---

# 24. Strategy Output

The strategy engine should return a structured result.

Example:

```json
{
  "asset": "BTC",
  "action": "BUY_40",
  "state": "ENTRY_PENDING",
  "price": 84600,
  "trend": "BULLISH",
  "position_pct": 0,
  "pullback_pct": 18.2,
  "local_high": 87400,
  "local_low": 79000,
  "drawdown_from_high_pct": 3.2,
  "reason": "Correction exceeded threshold, Higher Low confirmed, and price broke the previous local high.",
  "next_condition": "Wait for next entry confirmation."
}
```

Actions should be an enum:

```text
HOLD
BUY_40
BUY_30
BUY_30_FINAL
SELL_PROFIT
SELL_DRAWDOWN_10
SELL_DRAWDOWN_15
SELL_DRAWDOWN_20
WAIT
```

---

# 25. Explainability

Every signal must have a deterministic explanation.

Bad:

```text
AI thinks BTC will go up.
```

Good:

```text
BUY 40%

Reasons:
- Correction exceeded 10%
- Local low identified
- Higher Low confirmed
- Daily close broke previous local high
- Price is above SMA200
```

The explanation must be generated from strategy-engine facts.

No AI is required.

---

# 26. Portfolio Tracking

SQLite should store:

### assets

```text
id
symbol
quantity
average_entry_price
cash_allocated
updated_at
```

### transactions

```text
id
asset
type
quantity
price
timestamp
notes
```

Transaction types:

```text
BUY
SELL
DEPOSIT
WITHDRAWAL
```

The portfolio can initially be entered manually.

---

# 27. Market Data

Create an abstraction:

```go
type MarketDataProvider interface {
    GetCurrentPrice(symbol string) (float64, error)
    GetDailyCandles(symbol string, limit int) ([]Candle, error)
}
```

The first implementation can use a public API.

Later implementations can include:

```text
BinanceMarketDataProvider
```

Do not couple the strategy engine directly to Binance.

---

# 28. Future Binance Integration

Not part of MVP.

Future functionality:

- API key configuration
- read-only account balance
- read-only positions
- market data
- optional order execution later

Security requirements:

- never store API keys in SQLite as plaintext
- use macOS Keychain or secure OS storage
- start with read-only access

---

# 29. Future Wealthsimple Integration

Not part of MVP.

Wealthsimple may initially be represented by:

- manual portfolio input
- manual transaction entry

A future integration should implement the same portfolio interface used by the manual provider.

The strategy engine should not know whether portfolio data came from:

```text
Manual
Binance
Wealthsimple
```

---

# 30. Dashboard

Main page:

```text
====================================
CRYPTO STRATEGY ASSISTANT
====================================

BTC
------------------------------------
Price             $84,600
Trend             BULLISH
Position          0.12 BTC
Average Entry     $71,200
P/L               +18.5%
Local High        $87,400
Drawdown          -3.2%

ACTION
HOLD

Reason
Profit target reached, but price remains
within the current uptrend and no drawdown
exit threshold has triggered.

Next Trigger
-10% from local high
====================================

ETH
------------------------------------
...
```

---

# 31. History Page

Show historical strategy decisions.

Columns:

```text
Date
Asset
Price
State
Action
Reason
```

Example:

```text
Sep 24
BTC
84,600
PROFIT_PROTECTION
SELL 25%
+18.5% from average entry
```

This provides an audit trail.

---

# 32. Strategy Configuration Page

All important parameters must be editable.

Example:

```text
Correction
[10%]

Entry
[40%] [30%] [30%]

Profit Trigger
[10%]

Profit Sell
[25%]

Drawdown #1
[-10%] Sell [20%]

Drawdown #2
[-15%] Sell [30%]

Drawdown #3
[-20%] Sell [70%]

Trend Filter
[STRICT]
```

Changes should be stored in SQLite.

---

# 33. Alerts

Local notifications should trigger when:

- a new BUY signal appears
- a profit-taking signal appears
- a drawdown threshold is reached
- trend changes
- recovery breakout occurs

Do not send repeated alerts for the same event.

Store alert state in SQLite.

---

# 34. No AI in MVP

AI is deliberately excluded from the first version.

The decision engine should be:

```text
Input
  ↓
Rules
  ↓
State
  ↓
Action
  ↓
Explanation
```

AI may later be added for:

- natural-language explanations
- asking questions about strategy history
- summarizing market conditions
- explaining why a signal happened

AI must not silently override deterministic strategy rules.

---

# 35. No Backtesting in MVP

Backtesting is not technically required to build or run the application.

It is intentionally excluded from the first version.

However, the architecture should preserve the ability to add it later.

The strategy engine should therefore accept historical candle data without depending on real-time APIs.

Later:

```text
Historical Data
      ↓
Strategy Engine
      ↓
Simulated Portfolio
      ↓
Performance Report
```

Important:

The strategy parameters are hypotheses, not proven profitable parameters.

Backtesting should eventually be used before treating the strategy as validated.

---

# 36. No Paper Trading in MVP

Paper trading is also excluded.

The first version only needs:

```text
Market Data
+
Manual Portfolio
+
Strategy
+
Recommendations
```

The user manually decides whether to execute trades.

---

# 37. API

Suggested endpoints:

```text
GET /api/market/btc
GET /api/market/eth

GET /api/strategy/btc
GET /api/strategy/eth

GET /api/portfolio
PUT /api/portfolio

GET /api/history

GET /api/config
PUT /api/config
```

Optional:

```text
POST /api/portfolio/transaction
```

---

# 38. SQLite Tables

Minimum schema:

```text
portfolio
transactions
market_snapshots
strategy_states
strategy_events
settings
alerts
```

### market_snapshots

```text
id
asset
timestamp
price
open
high
low
close
volume
```

### strategy_states

```text
asset
state
local_high
local_low
higher_low
highest_price
drawdown_pct
updated_at
```

### strategy_events

```text
id
asset
timestamp
action
price
reason
state
```

---

# 39. Testing

The strategy engine must have unit tests.

Test cases:

### Correction

```text
100k → 89k
```

Should detect correction.

### No immediate buy

```text
100k → 80k
```

Should NOT generate BUY solely because price fell.

### Recovery

```text
100k → 80k → 85k → 82k → 90k
```

Should eventually generate a BUY after breakout, assuming trend filter passes.

### Profit

```text
Entry = 70k
Price = 77k
```

Should detect +10%.

### High-water mark

```text
70k → 90k → 81k
```

Drawdown should equal -10%.

### Drawdown sequence

```text
100k → 90k → 85k → 80k
```

Should trigger:

```text
20%
30%
70%
```

once each.

### New high reset

```text
100k → 90k → 110k → 99k
```

The new high must become 110k.

---

# 40. Important Engineering Rules

## Deterministic

Same inputs must produce the same result.

## No hidden state

All important state must be stored or derivable.

## Configurable

Thresholds must not be hard-coded throughout the codebase.

## Explainable

Every action must have explicit reasons.

## No over-trading

Daily candles and state transitions should prevent repeated signals.

## Separation of concerns

Keep:

```text
Market Data
Portfolio
Indicators
Strategy
Persistence
API
UI
```

separate.

---

# 41. Recommended Go Package Structure

```text
backend/
  cmd/
    server/

  internal/
    market/
    portfolio/
    indicators/
    strategy/
    storage/
    alerts/
    api/
    config/

  migrations/

  tests/
```

Strategy:

```text
internal/strategy/
    engine.go
    state.go
    signals.go
    pivots.go
    recovery.go
    exits.go
```

---

# 42. Recommended React Structure

```text
frontend/
  src/
    components/
    pages/
    hooks/
    api/
    types/
    utils/
```

Pages:

```text
Dashboard
History
Portfolio
Strategy Settings
```

---

# 43. Development Phases

## Phase 1 — Project Setup

- Tauri
- React
- TypeScript
- Go
- SQLite
- basic API
- basic dashboard

## Phase 2 — Market Data

- BTC daily candles
- ETH daily candles
- current price
- SMA50
- SMA200

## Phase 3 — Strategy Engine

Implement:

1. correction detection
2. pivots
3. local high/low
4. Higher Low
5. breakout
6. trend filter
7. entry state
8. profit-taking
9. high-water mark
10. drawdown exits
11. re-entry

## Phase 4 — Portfolio

- manual BTC position
- manual ETH position
- average entry
- cash
- transactions

## Phase 5 — Dashboard

- BTC card
- ETH card
- action
- reason
- state
- next trigger

## Phase 6 — Alerts

- local notifications
- duplicate prevention
- event history

## Phase 7 — Polish

- configuration UI
- error handling
- loading states
- data refresh
- persistence
- documentation

---

# 44. MVP Definition of Done

The MVP is complete when:

- [ ] App runs locally on macOS
- [ ] BTC data loads
- [ ] ETH data loads
- [ ] Daily candles are available
- [ ] SMA50 works
- [ ] SMA200 works
- [ ] Corrections are detected
- [ ] Local highs/lows are detected
- [ ] Higher Low detection works
- [ ] Breakouts are detected
- [ ] Trend filter works
- [ ] 40/30/30 entry state works
- [ ] Profit-taking works
- [ ] High-water mark works
- [ ] -10/-15/-20 drawdown rules work
- [ ] Re-entry logic works
- [ ] Manual portfolio works
- [ ] Dashboard works
- [ ] Strategy reasons are visible
- [ ] Signal history is stored
- [ ] Local alerts work
- [ ] Unit tests cover strategy transitions

Not required for MVP:

- [ ] Binance
- [ ] Wealthsimple
- [ ] AI
- [ ] Backtesting
- [ ] Paper trading
- [ ] Automated trading

---

# 45. Future Roadmap

After the MVP is stable:

### V2

- Binance read-only integration
- Wealthsimple integration if technically/API feasible
- richer charts
- transaction import
- automatic portfolio synchronization

### V3

- backtesting engine
- parameter optimization
- walk-forward testing
- out-of-sample validation
- performance metrics

### V4

- paper trading
- advanced alerts
- AI explanation layer

### V5

- optional automated execution with explicit user controls and strong safety mechanisms

---

# 46. Important Risk Disclaimer

This application is a personal decision-support tool.

The strategy is a rules-based hypothesis.

The rules:

- do not guarantee profits
- do not predict market direction
- can underperform buy-and-hold
- can generate false signals
- can suffer losses during volatile or sideways markets

Do not describe the strategy as proven or optimal without empirical validation.

---

# 47. First Implementation Priority

Do not start with:

- AI
- Binance
- Wealthsimple
- backtesting
- paper trading
- advanced charts

Start with the smallest useful loop:

```text
Market Data
    ↓
Strategy Engine
    ↓
BTC/ETH Action
    ↓
Reason
    ↓
Dashboard
```

Then add:

```text
Portfolio
    ↓
Alerts
    ↓
History
```

This produces a usable local application quickly while keeping the architecture ready for future integrations.
