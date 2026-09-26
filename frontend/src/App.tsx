import { useEffect, useRef, useState } from "react";
import { api } from "./api/client";
import { DashboardPage } from "./pages/DashboardPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "./types/api";

type Page = "dashboard" | "portfolio" | "history" | "settings";

const pages: Record<Page, { label: string; description?: string }> = {
  dashboard: { label: "Dashboard" },
  portfolio: { label: "Portfolio", description: "Manual portfolio editing is connected to the API. The form will be added with the portfolio phase." },
  history: { label: "History", description: "Signal history is persisted in SQLite and will appear here once the strategy engine emits events." },
  settings: { label: "Strategy settings", description: "BTC and ETH strategy defaults are persisted through the API. The configuration form follows with the strategy phase." },
};

export default function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const [portfolio, setPortfolio] = useState<Portfolio | null>(null);
  const [market, setMarket] = useState<Record<AssetSymbol, MarketSnapshot | null>>({ BTC: null, ETH: null });
  const [strategy, setStrategy] = useState<Record<AssetSymbol, StrategyResult | null>>({ BTC: null, ETH: null });
  const [isDashboardLoading, setIsDashboardLoading] = useState(true);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const dashboardRequested = useRef(false);

  useEffect(() => {
    // React Strict Mode replays effects in development. Strategy evaluation can
    // advance persisted state, so issue exactly one initial dashboard request.
    if (dashboardRequested.current) return;
    dashboardRequested.current = true;

    void Promise.allSettled([
      api.portfolio(),
      api.market("BTC"),
      api.market("ETH"),
      api.strategy("BTC"),
      api.strategy("ETH"),
    ]).then((results) => {
      const [portfolioResult, btcMarket, ethMarket, btcStrategy, ethStrategy] = results;
      if (portfolioResult.status === "fulfilled") setPortfolio(portfolioResult.value);
      if (btcMarket.status === "fulfilled") setMarket((current) => ({ ...current, BTC: btcMarket.value }));
      if (ethMarket.status === "fulfilled") setMarket((current) => ({ ...current, ETH: ethMarket.value }));
      if (btcStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, BTC: btcStrategy.value }));
      if (ethStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, ETH: ethStrategy.value }));

      const failures = results.filter((result): result is PromiseRejectedResult => result.status === "rejected");
      setConnectionError(failures.length > 0 ? failures.map((failure) => failure.reason instanceof Error ? failure.reason.message : "Request failed").join(" · ") : null);
      setIsDashboardLoading(false);
    });
  }, []);

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">◈</span><span>Crypto Strategy<br />Assistant</span></div>
        <nav aria-label="Main navigation">
          {(Object.keys(pages) as Page[]).map((key) => <button className={page === key ? "nav-item nav-item--active" : "nav-item"} key={key} onClick={() => setPage(key)}>{pages[key].label}</button>)}
        </nav>
        <p className={connectionError ? "connection connection--error" : "connection"}>{connectionError ? `Data unavailable: ${connectionError}` : "Local API connected"}</p>
      </aside>
      <section className="content">
        {page === "dashboard" ? <DashboardPage portfolio={portfolio} market={market} strategy={strategy} isLoading={isDashboardLoading} /> : <PlaceholderPage title={pages[page].label} description={pages[page].description!} />}
      </section>
    </main>
  );
}
