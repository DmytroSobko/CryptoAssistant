import { useEffect, useRef, useState } from "react";
import { PriceHistoryChart } from "./PriceHistoryChart";
import type { AssetPosition, AssetSymbol, MarketPriceHistory, MarketSnapshot, StrategyResult } from "../types/api";

interface AssetCardProps {
  asset: AssetSymbol;
  history: MarketPriceHistory;
  position?: AssetPosition;
  market: MarketSnapshot | null;
  strategy: StrategyResult | null;
  isLoading: boolean;
  onPriceHistoryLimitChange: (asset: AssetSymbol, recordLimit: number) => Promise<void>;
  onResolveRecommendation: (id: number, status: "EXECUTED" | "DISMISSED") => void;
}

function currency(value: number | undefined): string {
  return value === undefined ? "—" : new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(value);
}

function percent(value: number | undefined): string {
  if (value === undefined) return "—";
  return `${value > 0 ? "+" : ""}${value.toFixed(1)}%`;
}

function signedCurrencyDelta(value: number): string {
  return `${value > 0 ? "+" : "-"}${Math.abs(value).toFixed(2)}`;
}

function timestamp(value: string | undefined): string {
  if (!value) return "—";
  const date = new Date(value);
  return Number.isNaN(date.valueOf()) ? "—" : new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function actionLabel(result: StrategyResult | null): string {
  if (!result) return "WAIT";
  if (result.action.startsWith("BUY")) return `BUY ${result.actionPct}%`;
  if (result.action === "SELL_PROFIT" || result.action === "SELL_DRAWDOWN") {
    return `${result.action.replaceAll("_", " ")} ${result.actionPct}%`;
  }
  return result.action.replaceAll("_", " ");
}

function strategyName(strategyId: StrategyResult["strategyId"]): string {
  return strategyId === "DIP_ACCUMULATION" ? "Dip Accumulation" : "Recovery Breakout";
}

export function AssetCard({ asset, history, position, market, strategy, isLoading, onPriceHistoryLimitChange, onResolveRecommendation }: AssetCardProps) {
  const previousPrice = useRef<number | null>(market?.price ?? null);
  const priceDeltaSequence = useRef(0);
  const [priceDelta, setPriceDelta] = useState<{ id: number; value: number } | null>(null);
  const quantity = position?.quantity ?? 0;
  const entry = position?.averageEntryPrice ?? 0;
  const hasPosition = quantity > 0;
  const action = actionLabel(strategy);
  const actionClass = action.startsWith("BUY") ? "signal__action--buy" : action.startsWith("SELL") ? "signal__action--sell" : "";
  const status = isLoading ? "Loading data" : market ? "Market data loaded" : "Market data unavailable";

  useEffect(() => {
    const nextPrice = market?.price;
    if (nextPrice === undefined) {
      previousPrice.current = null;
      return;
    }

    const lastPrice = previousPrice.current;
    previousPrice.current = nextPrice;
    if (lastPrice === null) return;

    const delta = Math.round((nextPrice - lastPrice) * 100) / 100;
    if (delta === 0) return;

    priceDeltaSequence.current += 1;
    setPriceDelta({ id: priceDeltaSequence.current, value: delta });
    const timeout = window.setTimeout(() => setPriceDelta(null), 1900);
    return () => window.clearTimeout(timeout);
  }, [market?.price]);

  return (
    <article className="asset-card">
      <div className="asset-card__heading">
        <h2>{asset}</h2>
        <span className={market ? "badge badge--ready" : "badge badge--muted"}>{status}</span>
      </div>
      <dl className="metrics">
        <div><dt>Display price</dt><dd><span className="market-price__value">{currency(market?.price)}{priceDelta && <span aria-live="polite" className={priceDelta.value > 0 ? "price-delta price-delta--up" : "price-delta price-delta--down"} key={priceDelta.id}>{signedCurrencyDelta(priceDelta.value)}</span>}</span></dd></div>
        <div><dt>Trend</dt><dd>{strategy?.trend ?? "—"}</dd></div>
        <div><dt>Strategy state</dt><dd>{strategy?.state ?? "—"}</dd></div>
        <div><dt>Position</dt><dd>{quantity} {asset}</dd></div>
        <div><dt>Average entry</dt><dd>{entry ? currency(entry) : "—"}</dd></div>
        <div><dt>P/L</dt><dd>{hasPosition && strategy ? percent(strategy.profitLossPct) : "—"}</dd></div>
        <div><dt>Local high</dt><dd>{strategy?.localHigh ? currency(strategy.localHigh) : "—"}</dd></div>
        <div><dt>Drawdown</dt><dd>{hasPosition && strategy ? percent(strategy.drawdownFromHighPct) : "—"}</dd></div>
		<div><dt>Historical peak average</dt><dd>{strategy?.historicalPeakAverage ? currency(strategy.historicalPeakAverage) : "—"}</dd></div>
      </dl>
      <p className="market-timestamp">{market ? `Display price updated: ${timestamp(market.updatedAt)}` : "No display price has been loaded."}</p>
      <PriceHistoryChart asset={asset} history={history} onRecordLimitChange={onPriceHistoryLimitChange} />
      <section className="signal">
        <div className="signal__heading"><span className="eyebrow">Current action</span>{strategy && <span className="strategy-label">{strategyName(strategy.strategyId)}</span>}</div>
        <strong className={actionClass}>{action}</strong>
        <p>{strategy?.reason ?? (isLoading ? "Loading market data and the deterministic strategy." : "Completed daily market data is not available yet.")}</p>
        {strategy && strategy.price > 0 && <p><span className="muted">Decision basis:</span> {currency(strategy.price)} {strategy.intradayAlert ? "observed intraday price." : "completed daily close."}</p>}
        {strategy && strategy.breakEvenFloorGrossPrice && strategy.breakEvenFloorGrossPrice > 0 && <p className="signal__floor"><span className="muted">Break-even exit floor:</span> weighted average {currency(entry)} · do not sell below {currency(strategy.breakEvenFloorGrossPrice)} gross per {asset}.</p>}
        <p><span className="muted">Next trigger:</span> {strategy?.nextCondition ?? "Load completed daily candles."}</p>
        {strategy?.recommendationId && strategy.recommendationStatus === "PENDING" && <div className="form-actions"><button className="primary-button" onClick={() => onResolveRecommendation(strategy.recommendationId!, "EXECUTED")} type="button">Mark executed</button><button className="secondary-button" onClick={() => onResolveRecommendation(strategy.recommendationId!, "DISMISSED")} type="button">Dismiss</button></div>}
      </section>
    </article>
  );
}
