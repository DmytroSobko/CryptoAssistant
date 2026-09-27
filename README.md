# Crypto Strategy Assistant

A local macOS desktop decision-support application for a deterministic BTC and
ETH strategy. It is advisory-only: it does not connect to exchange accounts,
place orders, or execute trades.

## What it does

- Loads public Coinbase BTC/USD and ETH/USD market data at startup and every
  15 minutes.
- Persists the current display price and up to 250 completed UTC daily OHLC
  candles locally in SQLite.
- Evaluates a deterministic, explainable strategy using only completed daily
  candles: correction, recovery, Higher Low, breakout, trend filter, staged
  entries, profit protection, high-water marks, drawdown exits, and re-entry.
- Stores strategy state and idempotent BUY/SELL advisory history locally.
- Lets you manually maintain BTC/ETH positions, average entry prices, cash,
  and allocation values.
- Provides a dashboard, portfolio editor, history page, and independently
  configurable BTC and ETH strategy settings.

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
It records the asset, completed-candle timestamp, decision price, state,
action, and deterministic explanation.

### Strategy settings

BTC and ETH settings are independent. The Settings page validates pivot
windows, trend mode, staged-entry totals, profit triggers, and ordered
drawdown thresholds before saving. The API and storage layer independently
enforce the same constraints.

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
- Backtesting and paper trading.
- Manual transaction entry is not implemented; the app currently tracks the
  manually entered portfolio snapshot.

This software is for personal decision support only and is not financial
advice.
