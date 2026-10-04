# Crypto Strategy Assistant

A local macOS desktop decision-support application for a deterministic BTC and
ETH strategy. It is advisory-only: it does not connect to exchange accounts,
place orders, or execute trades.

## What it does

- Loads public Coinbase BTC/USD and ETH/USD market data at startup and every
  15 minutes.
- Persists the current display price and up to 250 completed UTC daily OHLC
  candles locally in SQLite.
- Evaluates one selected deterministic, explainable strategy per asset using
  only completed daily candles. `Recovery Breakout` is the original staged
  recovery strategy; `Dip Accumulation` confirms the same first entry, then
  averages down from that reference, takes configured profit, and protects
  later drawdown exits at break-even.
- Stores strategy state and idempotent BUY/SELL advisory history locally.
- Lets you manually maintain BTC/ETH positions, average entry prices, cash,
  and allocation values.
- Provides a dashboard, portfolio editor, history page, and independently
  configurable BTC and ETH strategy settings.
- Provides a separate, local-only backtesting workspace. It imports immutable
  BTC or ETH daily-candle CSV data and reports hypothetical strategy signals,
  simulated fills, equity, drawdown, fees, and buy-and-hold comparison.

## Important decision rule

The current market price is display-only. Every strategy decision is derived
from persisted, completed daily UTC candle closes. Refreshing the dashboard
never substitutes an intraday price for a daily-candle strategy input.

## Project layout

- `frontend/` — React, TypeScript, and Vite desktop UI
- `src-tauri/` — Tauri v2 macOS window and packaging configuration
- `backend/` — Go REST API, SQLite storage, market refresh, and strategy
  orchestration
- `backend/internal/strategy/` — pure deterministic strategy code with no
  HTTP, SQLite, Tauri, React, provider, exchange, or AI dependencies
- `backend/internal/backtest/` — pure historical simulator; it has no live
  portfolio, SQLite, HTTP, provider, or UI dependency
- `.documentation/` — MVP product specification

## Prerequisites

- Go 1.26+
- Node.js 20+ and npm
- Rust stable and Cargo for Tauri
- macOS Tauri system prerequisites; see the
  [Tauri setup guide](https://v2.tauri.app/start/prerequisites/)

## Run locally

Install dependencies once from the project root:

```sh
make bootstrap
```

Start the Go API, Vite development server, and Tauri window:

```sh
make dev
```

The local API listens on `http://localhost:8080` by default. Runtime data is
stored in `backend/data/cryptoassistant.db` and is ignored by Git.

### Runtime configuration

The backend reads environment variables; it does not automatically load an
`.env` file. Export any overrides before `make dev`:

| Variable | Default |
| --- | --- |
| `CRYPTO_ASSISTANT_API_ADDR` | `:8080` |
| `CRYPTO_ASSISTANT_DB_PATH` | `data/cryptoassistant.db` |
| `CRYPTO_ASSISTANT_MIGRATIONS_DIR` | `migrations` |
| `CRYPTO_ASSISTANT_MARKET_DATA_BASE_URL` | `https://api.exchange.coinbase.com` |
| `CRYPTO_ASSISTANT_MARKET_REFRESH_INTERVAL` | `15m` |

`.env.example` lists the usual local path and API-address values.

## User workflows

### Dashboard

The dashboard displays BTC and ETH display prices, trend, strategy state,
manual position, average entry, P/L, local high, drawdown, action, reason, and
next condition. Use **Refresh** to reload the local portfolio, current market
snapshots, and strategy results. It labels the completed daily close used as
the decision basis separately from the intraday display price.

### Portfolio

Use the Portfolio page to enter or update manual BTC/ETH quantities, average
entry prices, allocated cash, and the cash balance. Saving changes does not
trade; it refreshes the advisory snapshot so a manually observed position
change is reflected in strategy state.

### History

The History page is an audit trail of persisted BUY and SELL recommendations.
It records the asset, strategy, completed-candle timestamp, decision price,
state, action, and deterministic explanation.

### Strategy settings

BTC and ETH settings are independent. For each asset, select either
**Recovery Breakout** or **Dip Accumulation**; both configurations are saved
independently, while only the selected one generates future advisories.
Changing strategy starts a fresh advisory-state cycle for that asset and does
not change the other asset or execute a trade.

Dip Accumulation uses the normal recovery/trend gate for Entry 1. Entries 2
and 3 then occur at configured dips below the fixed first-entry reference
(defaults: 5% and 10%). It must first reach its configured profit target;
only then do drawdown exits activate, requiring estimated net proceeds to meet
the remaining weighted-average entry price. This can keep an underwater
position open for a long time; it does not guarantee a profitable campaign or
a manual market-order fill.

The profit target applies to the current holdings even after only Entry 1 or
Entry 2. For either strategy, signalling the profit sale cancels unused entry
stages until the position closes and a fresh cycle begins.

### Backtests

The **Backtests** page is separate from the Dashboard and History. It imports
one BTC or ETH historical candle set, runs one asset at a time, and previews a
result. Click **Save run** to keep it in the collapsible saved-runs archive.
A backtest never reads or changes the live portfolio, live
strategy state, live advisory History, or live market snapshots.

Choose a CSV file and give it a meaningful source label, such as `Coinbase
daily export`. After import, select its date range, starting cash, fees,
slippage, and a run-local strategy. It defaults to the asset's currently
selected assistance strategy, but you may choose either strategy for a test;
that choice and all snapshot edits do not save or overwrite the live BTC/ETH
strategy configuration. For Dip Accumulation, the first-entry reference is the
actual simulated Entry 1 fill. A later drawdown sale is shown as
`REJECTED BREAK EVEN FLOOR` if the simulated next-open net proceeds are below
the remaining average entry price; the position remains open.

Each result displays the data fingerprint, configuration snapshot, execution
assumptions, summary metrics, daily-close equity curve, every signal, and
every simulated trade. Rejected orders and end-of-data signals are retained in
the audit trail rather than being hidden.

#### Historical CSV format

Version 1 accepts a single asset per canonical CSV file with this exact
header:

```csv
timestamp,open,high,low,close,volume
2022-01-01T00:00:00Z,47686.81,47759.63,46288.49,47733.43,196.0
```

Requirements:

- Timestamps are RFC 3339 UTC midnight values, in strictly ascending order.
- Every UTC day must be present. Duplicate, unsorted, or missing days are
  rejected rather than filled in.
- `open`, `high`, `low`, and `close` must be finite positive values with a
  valid OHLC relationship. `volume` may be blank; when supplied it must be a
  finite non-negative value.
- At least 201 contiguous daily candles are required: 200 for SMA200 warm-up
  and at least one decision candle.

The import stores the original file's SHA-256, filename (without its local
path), source label, schema version, range, and candle count. A run also saves
a fingerprint of its selected candle values and range, so later data imports
cannot revise its historical result.

#### Simulation assumptions

- Decisions use completed UTC daily candles only. The signal is evaluated at
  the close of day `t`; an actionable signal is assumed filled at the open of
  day `t+1`.
- Starting position is cash only. Starting cash is required (the UI defaults
  to USD 10,000).
- Buys spend `ActionPct` of the cash available at the first buy of each cycle.
  That budget stays fixed across staged entries; later cycles reinvest prior
  net proceeds, including profits or losses. Buys are capped by available cash
  after fees. Sells use `ActionPct` of the quantity held when filled.
- Default fees are 10 bps per fill and default slippage is 5 bps per fill.
  Buy fills use `next open × (1 + slippage)` and sell fills use `next open ×
  (1 - slippage)`.
- The simulator never borrows, margins, or creates a negative cash/coin
  balance. Unfillable orders are recorded as rejected.
- Dip Accumulation compares a protected drawdown sale's known next-open fill,
  after simulated fees, with the remaining weighted-average entry. A failed
  comparison is recorded as `REJECTED_BREAK_EVEN_FLOOR`; it does not consume
  the drawdown level and may be eligible again later.
- Equity is marked at each completed daily close. Taxes are excluded.
- `ProfitTrigger2Pct` is retained only for compatibility with older snapshots.
  It has no sell rule and is not editable or validated.

Example walkthrough: import a validated BTC daily CSV, select the imported
set, leave the initial USD 10,000 / 10 bps / 5 bps defaults or enter explicit
alternatives, set an in-range period with the required warm-up, then select
**Run backtest**. Inspect the preview's fingerprint, assumptions, signals
and fills before comparing the reported return with buy-and-hold. These are
hypothetical historical outcomes, not a performance claim or investment
recommendation.

## API

| Method | Route | Purpose |
| --- | --- | --- |
| `GET` | `/healthz` | Local API health check |
| `GET` | `/api/market/{asset}` | Current display snapshot for BTC or ETH |
| `GET` | `/api/strategy/{asset}` | Deterministic completed-daily-candle advisory result |
| `GET` / `PUT` | `/api/portfolio` | Manual portfolio values |
| `GET` | `/api/history` | Persisted actionable strategy events |
| `GET` | `/api/config` | BTC and ETH strategy settings |
| `PUT` | `/api/config/{asset}` | Validated settings for BTC or ETH |
| `GET` | `/api/strategy-settings` | Full selected strategy profile for BTC and ETH |
| `PUT` | `/api/strategy-settings/{asset}` | Save one asset's selected strategy and both configurations |
| `POST` | `/api/backtest/candle-sets` | Import a validated historical CSV as an immutable candle set |
| `GET` | `/api/backtest/candle-sets?asset=BTC` | List imported BTC or ETH candle sets |
| `POST` | `/api/backtests` | Run and save a synchronous hypothetical simulation |
| `POST` | `/api/backtests/preview` | Run a simulation without saving it |
| `GET` | `/api/backtests` | List saved backtest summaries |
| `GET` | `/api/backtests/{id}` | Retrieve one saved backtest and its audit data |

## Verification

Run the backend suite:

```sh
make test
```

When the default Go build cache is not writable, use the isolated cache command
used for this project:

```sh
cd backend
GOCACHE=/private/tmp/crypto-assistant-go-cache go test ./...
GOCACHE=/private/tmp/crypto-assistant-go-cache go vet ./...
```

The frontend can be checked after Node/npm dependencies are installed:

```sh
cd frontend
npm run build
```

## Deliberately not included

- Local alerts and notifications are deferred.
- Exchange/account integrations, autonomous trading, or order execution.
- AI-generated signals or AI overrides of deterministic rules.
- Paper trading, parameter optimisation, walk-forward analysis, or automated
  "best settings" selection.
- Manual transaction entry is not implemented; the app currently tracks the
  manually entered portfolio snapshot.

This software is for personal decision support only and is not financial
advice.
