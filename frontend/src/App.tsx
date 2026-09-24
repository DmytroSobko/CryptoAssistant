import { useEffect, useState } from "react";
import { api } from "./api/client";
import { DashboardPage } from "./pages/DashboardPage";
import { PlaceholderPage } from "./pages/PlaceholderPage";
import type { Portfolio } from "./types/api";

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
  const [connectionError, setConnectionError] = useState<string | null>(null);

  useEffect(() => {
    api.portfolio().then(setPortfolio).catch((error: Error) => setConnectionError(error.message));
  }, []);

  return (
    <main className="app-shell">
      <aside className="sidebar">
        <div className="brand"><span className="brand-mark">◈</span><span>Crypto Strategy<br />Assistant</span></div>
        <nav aria-label="Main navigation">
          {(Object.keys(pages) as Page[]).map((key) => <button className={page === key ? "nav-item nav-item--active" : "nav-item"} key={key} onClick={() => setPage(key)}>{pages[key].label}</button>)}
        </nav>
        <p className={connectionError ? "connection connection--error" : "connection"}>{connectionError ? `API offline: ${connectionError}` : "Local API connected"}</p>
      </aside>
      <section className="content">
        {page === "dashboard" ? <DashboardPage portfolio={portfolio} /> : <PlaceholderPage title={pages[page].label} description={pages[page].description!} />}
      </section>
    </main>
  );
}

