import type { BacktestSignal, BacktestTrade } from "../types/api";

const usd = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 2 });

export function BacktestTradeTable({ signals, trades }: { signals: BacktestSignal[]; trades: BacktestTrade[] }) {
  return (
    <div className="backtest-tables">
      <section className="backtest-results-section">
        <div className="backtest-section-heading"><div><p className="eyebrow">Decision audit</p><h2>Signals</h2></div></div>
        <div className="history-table-wrap"><table className="history-table backtest-table"><thead><tr><th>Date</th><th>Close</th><th>Action</th><th>State</th><th>Execution</th><th>Net floor</th><th>Reason</th></tr></thead><tbody>
          {signals.map((signal) => <tr key={signal.sequence}><td>{signal.timestamp.slice(0, 10)}</td><td>{usd.format(signal.decisionClose)}</td><td className="history-action">{formatAction(signal.action, signal.actionPct)}</td><td>{signal.strategyState.replaceAll("_", " ")}</td><td><Status value={signal.orderStatus} /></td><td>{floor(signal.breakEvenFloorNetPrice)}</td><td>{signal.reason}<small>{signal.nextCondition}</small></td></tr>)}
        </tbody></table></div>
      </section>
      <section className="backtest-results-section">
        <div className="backtest-section-heading"><div><p className="eyebrow">Assume-filled model</p><h2>Trades</h2></div></div>
        {trades.length === 0 ? <p className="muted">No simulated fills were produced for this period.</p> : <div className="history-table-wrap"><table className="history-table backtest-table"><thead><tr><th>Signal</th><th>Fill</th><th>Side</th><th>Quantity</th><th>Fill price</th><th>Fee</th><th>Net floor</th><th>Status</th></tr></thead><tbody>
          {trades.map((trade) => <tr key={trade.sequence}><td>{trade.signalTimestamp.slice(0, 10)}</td><td>{trade.executionTimestamp ? trade.executionTimestamp.slice(0, 10) : "—"}</td><td>{trade.side}</td><td>{trade.quantity.toFixed(8)}</td><td>{trade.fillPrice ? usd.format(trade.fillPrice) : "—"}</td><td>{usd.format(trade.feeUsd)}</td><td>{floor(trade.breakEvenFloorNetPrice)}</td><td><Status value={trade.status} /><small>{trade.reason}</small></td></tr>)}
        </tbody></table></div>}
      </section>
    </div>
  );
}

function Status({ value }: { value: string }) { return <span className={value === "EXECUTED" ? "backtest-status backtest-status--success" : value === "REJECTED_BREAK_EVEN_FLOOR" ? "backtest-status backtest-status--blocked" : value === "NO_ORDER" ? "backtest-status" : "backtest-status backtest-status--warning"}>{value.replaceAll("_", " ")}</span>; }
function floor(value: number | undefined) { return value && value > 0 ? usd.format(value) : "—"; }
function formatAction(action: string, percentage: number) { return percentage > 0 ? `${action.replaceAll("_", " ")} ${percentage}%` : action.replaceAll("_", " "); }
