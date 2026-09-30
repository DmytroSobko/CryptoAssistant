# Historical backtest CSVs

These four files contain public Coinbase Exchange daily OHLCV candles for
`BTC-USD` and `ETH-USD`:

- `btc-usd-daily-5y-2021-09-30-to-2026-09-29.csv`
- `btc-usd-daily-10y-2016-09-30-to-2026-09-29.csv`
- `eth-usd-daily-5y-2021-09-30-to-2026-09-29.csv`
- `eth-usd-daily-10y-2016-09-30-to-2026-09-29.csv`

Source: Coinbase Exchange public daily candle endpoint. The files deliberately
end on 2026-09-29 UTC so they contain completed daily candles only; the
in-progress 2026-09-30 candle is excluded.

Use `Coinbase Exchange public daily candles` as the Backtests-page source
label when importing. The five-year data sets are the latest 1,826 completed
UTC days; the ten-year data sets contain 3,652 completed UTC days.
