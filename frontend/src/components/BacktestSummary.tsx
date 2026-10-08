import type { AssetSymbol, BacktestResult } from "../types/api";

const usd = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 });
const percent = (value: number) => `${value >= 0 ? "+" : ""}${value.toFixed(2)}%`;

export function BacktestSummary({ asset, result }: { asset: AssetSymbol; result: BacktestResult }) {
  const { summary, assumptions } = result;
  const strategyName = result.request.strategyConfig.strategyId === "DIP_ACCUMULATION" ? "Dip Accumulation" : "Recovery Breakout";
  const isDipAccumulation = result.request.strategyConfig.strategyId === "DIP_ACCUMULATION";
  return (
    <section className="backtest-results-section">
      <div className="backtest-section-heading"><div><p className="eyebrow">Completed simulation</p><h2>Results</h2></div><p className="backtest-fingerprint">Data fingerprint: <code>{result.dataFingerprint}</code></p></div>
      <div className="backtest-metrics">
        <Metric label="Starting capital" value={usd.format(summary.startingCashUsd)} />
        <Metric label="Ending equity" value={usd.format(summary.endingEquityUsd)} />
        <Metric label="Net P/L" tone={summary.netProfitLossUsd >= 0 ? "positive" : "negative"} value={usd.format(summary.netProfitLossUsd)} />
        <Metric label="Return" tone={summary.returnPct >= 0 ? "positive" : "negative"} value={percent(summary.returnPct)} />
        <Metric label="Maximum drawdown" tone="negative" value={percent(summary.maximumDrawdownPct)} />
        <Metric label="Total fees" value={usd.format(summary.totalFeesUsd)} />
        <Metric label="Filled orders" value={String(summary.executedTradeCount)} />
        <Metric label="Buy & hold return" tone={summary.buyAndHoldReturnPct >= 0 ? "positive" : "negative"} value={percent(summary.buyAndHoldReturnPct)} />
        {isDipAccumulation && <Metric label="Protected exits blocked" tone={summary.breakEvenFloorBlockedTradeCount > 0 ? "negative" : undefined} value={String(summary.breakEvenFloorBlockedTradeCount)} />}
      </div>
      <div className="backtest-result-note">
        <span>{asset} · {result.request.start.slice(0, 10)} to {result.request.end.slice(0, 10)}</span>
        <span>{assumptions.executionModel.replaceAll("_", " ")} · fees {assumptions.feeBps} bps · slippage {assumptions.slippageBps} bps</span>
        <span>Strategy: {strategyName} · {result.request.strategyConfig.trendMode} · entries {result.request.strategyConfig.entry1Pct}/{result.request.strategyConfig.entry2Pct}/{result.request.strategyConfig.entry3Pct}%</span>
        {assumptions.athEntryOverride && <span>{assumptions.athEntryOverride}</span>}
        {assumptions.recoveryEntryPeakGate && <span>{assumptions.recoveryEntryPeakGate}</span>}
        <span>Taxes excluded · {summary.rejectedTradeCount} rejected · {summary.unexecutedTradeCount} unexecuted</span>
        {isDipAccumulation && <><span>{assumptions.referencePricePolicy}</span><span>{assumptions.breakEvenExitFloor}</span></>}
      </div>
    </section>
  );
}

function Metric({ label, value, tone }: { label: string; value: string; tone?: "positive" | "negative" }) {
  return <div className="backtest-metric"><span>{label}</span><strong className={tone ? `metric--${tone}` : ""}>{value}</strong></div>;
}
