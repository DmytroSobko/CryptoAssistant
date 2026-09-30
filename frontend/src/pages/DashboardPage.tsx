import { AssetCard } from "../components/AssetCard";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "../types/api";

interface DashboardPageProps {
  portfolio: Portfolio | null;
  market: Record<AssetSymbol, MarketSnapshot | null>;
  strategy: Record<AssetSymbol, StrategyResult | null>;
  isLoading: boolean;
  lastRefreshedAt: Date | null;
  onRefresh: (refreshMarketData?: boolean) => Promise<void>;
}

function refreshTime(value: Date | null): string {
  return value ? new Intl.DateTimeFormat("en-US", { timeStyle: "medium" }).format(value) : "Not checked yet";
}

export function DashboardPage({ portfolio, market, strategy, isLoading, lastRefreshedAt, onRefresh }: DashboardPageProps) {
  const position = (symbol: "BTC" | "ETH") => portfolio?.assets.find((asset) => asset.symbol === symbol);
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
            <button className="secondary-button" disabled={isLoading} onClick={() => { void onRefresh(true); }} type="button">{isLoading ? "Refreshing…" : "Refresh"}</button>
          </div>
        </div>
      </header>
      <div className="asset-grid">
        <AssetCard asset="BTC" position={position("BTC")} market={market.BTC} strategy={strategy.BTC} isLoading={isLoading} />
        <AssetCard asset="ETH" position={position("ETH")} market={market.ETH} strategy={strategy.ETH} isLoading={isLoading} />
      </div>
    </>
  );
}
