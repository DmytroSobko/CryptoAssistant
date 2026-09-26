import { AssetCard } from "../components/AssetCard";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "../types/api";

interface DashboardPageProps {
  portfolio: Portfolio | null;
  market: Record<AssetSymbol, MarketSnapshot | null>;
  strategy: Record<AssetSymbol, StrategyResult | null>;
  isLoading: boolean;
}

export function DashboardPage({ portfolio, market, strategy, isLoading }: DashboardPageProps) {
  const position = (symbol: "BTC" | "ETH") => portfolio?.assets.find((asset) => asset.symbol === symbol);
  return (
    <>
      <header className="page-header">
        <div>
          <p className="eyebrow">Local decision support</p>
          <h1>Market overview</h1>
        </div>
        <p className="cash-balance">Cash balance: <strong>${(portfolio?.cashBalance ?? 0).toLocaleString()}</strong></p>
      </header>
      <div className="asset-grid">
        <AssetCard asset="BTC" position={position("BTC")} market={market.BTC} strategy={strategy.BTC} isLoading={isLoading} />
        <AssetCard asset="ETH" position={position("ETH")} market={market.ETH} strategy={strategy.ETH} isLoading={isLoading} />
      </div>
    </>
  );
}
