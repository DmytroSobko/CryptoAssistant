import { useEffect, useRef, useState } from "react";
import { api } from "./api/client";
import { DashboardPage } from "./pages/DashboardPage";
import { HistoryPage } from "./pages/HistoryPage";
import { PortfolioPage } from "./pages/PortfolioPage";
import { StrategySettingsPage } from "./pages/StrategySettingsPage";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "./types/api";

type Page = "dashboard" | "portfolio" | "history" | "settings";

const pages: Record<Page, { label: string }> = {
  dashboard: { label: "Dashboard" },
  portfolio: { label: "Portfolio" },
  history: { label: "History" },
  settings: { label: "Strategy settings" },
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

  function handlePortfolioSaved(savedPortfolio: Portfolio) {
    setPortfolio(savedPortfolio);
    // A manual position change is an observed strategy fact (for example, a
    // user closing a position). Re-evaluate from completed daily candles.
    void Promise.allSettled([api.strategy("BTC"), api.strategy("ETH")]).then((results) => {
      const [btcStrategy, ethStrategy] = results;
      if (btcStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, BTC: btcStrategy.value }));
      if (ethStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, ETH: ethStrategy.value }));
    });
  }

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
        {page === "dashboard" && <DashboardPage portfolio={portfolio} market={market} strategy={strategy} isLoading={isDashboardLoading} />}
        {page === "portfolio" && <PortfolioPage isLoading={isDashboardLoading} onSaved={handlePortfolioSaved} portfolio={portfolio} />}
        {page === "history" && <HistoryPage />}
        {page === "settings" && <StrategySettingsPage />}
      </section>
    </main>
  );
}
