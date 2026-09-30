import { useCallback, useEffect, useRef, useState } from "react";
import { api } from "./api/client";
import { DashboardPage } from "./pages/DashboardPage";
import { BacktestPage } from "./pages/BacktestPage";
import { HistoryPage } from "./pages/HistoryPage";
import { PortfolioPage } from "./pages/PortfolioPage";
import { StrategySettingsPage } from "./pages/StrategySettingsPage";
import type { AssetSymbol, MarketSnapshot, Portfolio, StrategyResult } from "./types/api";

type Page = "dashboard" | "portfolio" | "history" | "settings" | "backtests";

const pages: Record<Page, { label: string }> = {
  dashboard: { label: "Dashboard" },
  portfolio: { label: "Portfolio" },
  history: { label: "History" },
  settings: { label: "Strategy settings" },
  backtests: { label: "Backtests" },
};

export default function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const [portfolio, setPortfolio] = useState<Portfolio | null>(null);
  const [market, setMarket] = useState<Record<AssetSymbol, MarketSnapshot | null>>({ BTC: null, ETH: null });
  const [strategy, setStrategy] = useState<Record<AssetSymbol, StrategyResult | null>>({ BTC: null, ETH: null });
  const [isDashboardLoading, setIsDashboardLoading] = useState(true);
  const [lastDashboardRefresh, setLastDashboardRefresh] = useState<Date | null>(null);
  const [connectionError, setConnectionError] = useState<string | null>(null);
  const dashboardRequested = useRef(false);
  const dashboardRefreshInProgress = useRef(false);

  const refreshDashboard = useCallback(async (refreshMarketData = false) => {
    if (dashboardRefreshInProgress.current) return;
    dashboardRefreshInProgress.current = true;
    setIsDashboardLoading(true);
    setConnectionError(null);
    try {
      const failures: unknown[] = [];
      if (refreshMarketData) {
        try {
          await api.refreshMarket();
        } catch (error) {
          failures.push(error);
        }
      }
      const results = await Promise.allSettled([
        api.portfolio(),
        api.market("BTC"),
        api.market("ETH"),
        api.strategy("BTC"),
        api.strategy("ETH"),
      ]);
      const [portfolioResult, btcMarket, ethMarket, btcStrategy, ethStrategy] = results;
      if (portfolioResult.status === "fulfilled") setPortfolio(portfolioResult.value);
      if (btcMarket.status === "fulfilled") setMarket((current) => ({ ...current, BTC: btcMarket.value }));
      if (ethMarket.status === "fulfilled") setMarket((current) => ({ ...current, ETH: ethMarket.value }));
      if (btcStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, BTC: btcStrategy.value }));
      if (ethStrategy.status === "fulfilled") setStrategy((current) => ({ ...current, ETH: ethStrategy.value }));

      for (const result of results) {
        if (result.status === "rejected") failures.push(result.reason);
      }
      setConnectionError(failures.length > 0 ? failures.map((failure) => failure instanceof Error ? failure.message : "Request failed").join(" · ") : null);
      setLastDashboardRefresh(new Date());
    } finally {
      dashboardRefreshInProgress.current = false;
      setIsDashboardLoading(false);
    }
  }, []);

  useEffect(() => {
    // React Strict Mode replays effects in development. Strategy evaluation can
    // advance persisted state, so issue exactly one initial dashboard request.
    if (dashboardRequested.current) return;
    dashboardRequested.current = true;

    void refreshDashboard();
  }, [refreshDashboard]);

  function handlePortfolioSaved(savedPortfolio: Portfolio) {
    setPortfolio(savedPortfolio);
    // A manual position change is an observed strategy fact (for example, a
    // user closing a position), so refresh the entire advisory snapshot.
    void refreshDashboard();
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
        {page === "dashboard" && <DashboardPage isLoading={isDashboardLoading} lastRefreshedAt={lastDashboardRefresh} market={market} onRefresh={refreshDashboard} portfolio={portfolio} strategy={strategy} />}
        {page === "portfolio" && <PortfolioPage isLoading={isDashboardLoading} onSaved={handlePortfolioSaved} portfolio={portfolio} />}
        {page === "history" && <HistoryPage />}
        {page === "settings" && <StrategySettingsPage />}
        {page === "backtests" && <BacktestPage />}
      </section>
    </main>
  );
}
