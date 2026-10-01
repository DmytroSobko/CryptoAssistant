import { useEffect, useState } from "react";
import { api } from "../api/client";
import type { StrategyEvent } from "../types/api";

function formatDate(timestamp: string): string {
  const date = new Date(timestamp);
  if (Number.isNaN(date.valueOf())) return "—";
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function formatPrice(price: number): string {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 }).format(price);
}

function formatAction(action: string): string {
  switch (action) {
    case "BUY_40": return "BUY 40%";
    case "BUY_30": return "BUY 30%";
    case "BUY_30_FINAL": return "BUY 30%";
    default: return action.replaceAll("_", " ");
  }
}

function strategyName(strategyId: StrategyEvent["strategyId"]): string {
  return strategyId === "DIP_ACCUMULATION" ? "Dip Accumulation" : "Recovery Breakout";
}

export function HistoryPage() {
  const [events, setEvents] = useState<StrategyEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    let active = true;
    setIsLoading(true);
    setError(null);
    void api.history().then(
      (loadedEvents) => {
        if (!active) return;
        setEvents(loadedEvents);
        setIsLoading(false);
      },
      (loadError: unknown) => {
        if (!active) return;
        setError(loadError instanceof Error ? loadError.message : "Could not load strategy history.");
        setIsLoading(false);
      },
    );
    return () => { active = false; };
  }, [reloadKey]);

  return (
    <section className="history-page">
      <header className="page-header">
        <div>
          <p className="eyebrow">Advisory audit trail</p>
          <h1>History</h1>
        </div>
        <button className="secondary-button" disabled={isLoading} onClick={() => setReloadKey((key) => key + 1)} type="button">{isLoading ? "Loading…" : "Refresh"}</button>
      </header>

      {error && <p className="form-message form-message--error" role="alert">{error}</p>}
      {!error && !isLoading && events.length === 0 && <section className="placeholder"><h2>No advisory actions yet</h2><p>New BUY and SELL recommendations from completed daily candles will appear here.</p></section>}
      {!error && events.length > 0 && (
        <div className="history-table-wrap">
          <table className="history-table">
            <thead><tr><th>Date</th><th>Asset</th><th>Strategy</th><th>Price</th><th>State</th><th>Action</th><th>Reason</th></tr></thead>
            <tbody>
              {events.map((event) => (
                <tr key={event.id}>
                  <td>{formatDate(event.timestamp)}</td>
                  <td>{event.asset}</td>
                  <td>{strategyName(event.strategyId)}</td>
                  <td>{formatPrice(event.price)}</td>
                  <td>{event.state.replaceAll("_", " ")}</td>
                  <td className="history-action">{formatAction(event.action)}</td>
                  <td>{event.reason}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
