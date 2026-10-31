import { useEffect, useState, type FormEvent } from "react";
import { AssetCard } from "../components/AssetCard";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "../types/api";

interface DashboardPageProps {
  portfolio: Portfolio | null;
  market: Record<AssetSymbol, MarketSnapshot | null>;
  strategy: Record<AssetSymbol, StrategyResult | null>;
  isLoading: boolean;
  isRefreshing: boolean;
  lastRefreshedAt: Date | null;
  priceRefreshIntervalSeconds: number;
  onPriceRefreshIntervalChange: (seconds: number) => Promise<void>;
  onRefresh: (refreshMarketData?: boolean) => Promise<void>;
  onResolveRecommendation: (id: number, status: "EXECUTED" | "DISMISSED") => void;
}

function refreshTime(value: Date | null): string {
  return value ? new Intl.DateTimeFormat("en-US", { timeStyle: "medium" }).format(value) : "Not checked yet";
}

export function DashboardPage({ portfolio, market, strategy, isLoading, isRefreshing, lastRefreshedAt, priceRefreshIntervalSeconds, onPriceRefreshIntervalChange, onRefresh, onResolveRecommendation }: DashboardPageProps) {
  const [intervalDraft, setIntervalDraft] = useState(String(priceRefreshIntervalSeconds));
  const [intervalMessage, setIntervalMessage] = useState<string | null>(null);
  const [intervalError, setIntervalError] = useState(false);
  const [isSavingInterval, setIsSavingInterval] = useState(false);
  const position = (symbol: "BTC" | "ETH") => portfolio?.assets.find((asset) => asset.symbol === symbol);

  useEffect(() => {
    setIntervalDraft(String(priceRefreshIntervalSeconds));
  }, [priceRefreshIntervalSeconds]);

  async function saveInterval(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const seconds = Number(intervalDraft);
    if (!Number.isInteger(seconds) || seconds < 1 || seconds > 86400) {
      setIntervalError(true);
      setIntervalMessage("Enter a whole number from 1 to 86,400 seconds.");
      return;
    }
    setIsSavingInterval(true);
    setIntervalMessage(null);
    try {
      await onPriceRefreshIntervalChange(seconds);
      setIntervalError(false);
      setIntervalMessage(`Auto-refresh updated to every ${seconds} seconds.`);
    } catch (error) {
      setIntervalError(true);
      setIntervalMessage(error instanceof Error ? error.message : "Could not save the auto-refresh interval.");
    } finally {
      setIsSavingInterval(false);
    }
  }

  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">Local decision support</p>
          <h1>Market overview</h1>
        </div>
        <div className="dashboard-summary">
          <p className="cash-balance">Cash balance: <strong>${(portfolio?.cashBalance ?? 0).toLocaleString()}</strong></p>
          <div className="dashboard-refresh">
            <span>Last checked: {refreshTime(lastRefreshedAt)}</span>
            <form className="dashboard-interval" onSubmit={(event) => { void saveInterval(event); }}>
              <label htmlFor="price-refresh-interval">Auto-refresh</label>
              <input disabled={isSavingInterval} id="price-refresh-interval" inputMode="numeric" max="86400" min="1" onChange={(event) => { setIntervalDraft(event.target.value); setIntervalMessage(null); }} step="1" type="number" value={intervalDraft} />
              <span>sec</span>
              <button className="secondary-button" disabled={isSavingInterval} type="submit">{isSavingInterval ? "Saving…" : "Apply"}</button>
            </form>
            <button className="secondary-button" disabled={isRefreshing} onClick={() => { void onRefresh(true); }} type="button">{isRefreshing ? "Refreshing…" : "Refresh"}</button>
          </div>
          {intervalMessage && <p className={intervalError ? "dashboard-interval-message dashboard-interval-message--error" : "dashboard-interval-message"}>{intervalMessage}</p>}
        </div>
      </header>
      <div className="asset-grid">
        <AssetCard asset="BTC" position={position("BTC")} market={market.BTC} strategy={strategy.BTC} isLoading={isLoading} onResolveRecommendation={onResolveRecommendation} />
        <AssetCard asset="ETH" position={position("ETH")} market={market.ETH} strategy={strategy.ETH} isLoading={isLoading} onResolveRecommendation={onResolveRecommendation} />
      </div>
    </>
  );
}
