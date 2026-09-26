import type { AssetPosition, AssetSymbol, MarketSnapshot, StrategyResult } from "../types/api";

interface AssetCardProps {
  asset: AssetSymbol;
  position?: AssetPosition;
  market: MarketSnapshot | null;
  strategy: StrategyResult | null;
  isLoading: boolean;
}

function currency(value: number | undefined): string {
  return value === undefined ? "—" : new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(value);
}

function percent(value: number | undefined): string {
  if (value === undefined) return "—";
  return `${value > 0 ? "+" : ""}${value.toFixed(1)}%`;
}

function actionLabel(result: StrategyResult | null): string {
  if (!result) return "WAIT";
  if (result.action === "BUY" || result.action === "SELL_PROFIT" || result.action === "SELL_DRAWDOWN") {
    return `${result.action.replaceAll("_", " ")} ${result.actionPct}%`;
  }
  return result.action.replace("BUY_40", "BUY 40%").replace("BUY_30_FINAL", "BUY 30%").replace("BUY_30", "BUY 30%").replaceAll("_", " ");
}

export function AssetCard({ asset, position, market, strategy, isLoading }: AssetCardProps) {
  const quantity = position?.quantity ?? 0;
  const entry = position?.averageEntryPrice ?? 0;
  const hasPosition = quantity > 0;
  const action = actionLabel(strategy);
  const actionClass = action.startsWith("BUY") ? "signal__action--buy" : action.startsWith("SELL") ? "signal__action--sell" : "";
  const status = isLoading ? "Loading data" : market ? "Market data loaded" : "Market data unavailable";

  return (
    <article className="asset-card">
      <div className="asset-card__heading">
        <h2>{asset}</h2>
        <span className={market ? "badge badge--ready" : "badge badge--muted"}>{status}</span>
      </div>
      <dl className="metrics">
        <div><dt>Price</dt><dd>{currency(market?.price)}</dd></div>
        <div><dt>Trend</dt><dd>{strategy?.trend ?? "—"}</dd></div>
        <div><dt>Strategy state</dt><dd>{strategy?.state ?? "—"}</dd></div>
        <div><dt>Position</dt><dd>{quantity} {asset}</dd></div>
        <div><dt>Average entry</dt><dd>{entry ? currency(entry) : "—"}</dd></div>
        <div><dt>P/L</dt><dd>{hasPosition && strategy ? percent(strategy.profitLossPct) : "—"}</dd></div>
        <div><dt>Local high</dt><dd>{strategy?.localHigh ? currency(strategy.localHigh) : "—"}</dd></div>
        <div><dt>Drawdown</dt><dd>{hasPosition && strategy ? percent(strategy.drawdownFromHighPct) : "—"}</dd></div>
      </dl>
      <section className="signal">
        <span className="eyebrow">Current action</span>
        <strong className={actionClass}>{action}</strong>
        <p>{strategy?.reason ?? (isLoading ? "Loading market data and the deterministic strategy." : "Completed daily market data is not available yet.")}</p>
        <p><span className="muted">Next trigger:</span> {strategy?.nextCondition ?? "Load completed daily candles."}</p>
      </section>
    </article>
  );
}
