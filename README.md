# Crypto Strategy Assistant

A local desktop decision-support application for a deterministic BTC/ETH
strategy. It is advisory-only: it never connects to an exchange or executes a
trade.

## Project layout

- `frontend/` — React, TypeScript, and Vite dashboard
- `src-tauri/` — native macOS desktop window and packaging configuration
- `backend/` — Go REST API, SQLite storage, migrations, and future strategy engine
- `.documentation/` — product and MVP specification

The strategy package has no dependency on Tauri, React, HTTP, SQLite, or a
market-data vendor. This keeps the eventual rules engine deterministic and
unit-testable.

## Prerequisites

- Go 1.26+
- Node.js 20+ and npm
- Rust stable and Cargo (needed by Tauri)
- macOS Tauri system prerequisites: [Tauri setup guide](https://v2.tauri.app/start/prerequisites/)

## Run locally

From the project root, install dependencies once:

```sh
make bootstrap
```

Then use the single development command:

```sh
make dev
```

This starts the Go API at `http://localhost:8080`, the Vite development server,
and the Tauri desktop window. The SQLite database is created at
`backend/data/cryptoassistant.db`; it is local runtime data and is ignored by
Git.

Run backend tests with:

```sh
make test
```

Use the variables in `.env.example` only if you need to override the API
address, database path, or migrations location; export them in the shell before
running `make dev`.

## Current foundation

The skeleton already includes versioned SQLite migrations; manual portfolio
read/write endpoints; per-asset strategy configuration defaults persisted in
SQLite; health, market, history, and strategy endpoint shapes; and a desktop
dashboard shell. Market providers and the deterministic strategy transition
rules are deliberately pending implementation, so the UI currently shows an
explicit `WAIT` state rather than inventing a signal.
