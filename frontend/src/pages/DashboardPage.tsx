import { AssetCard } from "../components/AssetCard";
import type { Portfolio } from "../types/api";

interface DashboardPageProps {
  portfolio: Portfolio | null;
}

export function DashboardPage({ portfolio }: DashboardPageProps) {
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
        <AssetCard asset="BTC" position={position("BTC")} />
        <AssetCard asset="ETH" position={position("ETH")} />
      </div>
    </>
  );
}

