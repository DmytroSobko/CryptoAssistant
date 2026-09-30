import type { BacktestEquityPoint } from "../types/api";

const usd = new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 });

export function BacktestEquityChart({ points }: { points: BacktestEquityPoint[] }) {
  if (points.length === 0) return null;
  const values = points.map((point) => point.equityUsd);
  const minimum = Math.min(...values);
  const maximum = Math.max(...values);
  const span = Math.max(maximum - minimum, 1);
  const width = 760;
  const height = 220;
  const padding = 14;
  const plot = points.map((point, index) => {
    const x = padding + (index / Math.max(points.length - 1, 1)) * (width - padding * 2);
    const y = height - padding - ((point.equityUsd - minimum) / span) * (height - padding * 2);
    return `${x},${y}`;
  }).join(" ");
  return (
    <section className="backtest-results-section">
      <div className="backtest-section-heading"><div><p className="eyebrow">Daily close valuation</p><h2>Equity curve</h2></div><p className="muted">{usd.format(minimum)} — {usd.format(maximum)}</p></div>
      <div className="equity-chart-wrap">
        <svg aria-label="Daily close equity curve" className="equity-chart" role="img" viewBox={`0 0 ${width} ${height}`}>
          <line x1={padding} x2={width - padding} y1={padding} y2={padding} className="equity-chart__grid" />
          <line x1={padding} x2={width - padding} y1={height / 2} y2={height / 2} className="equity-chart__grid" />
          <line x1={padding} x2={width - padding} y1={height - padding} y2={height - padding} className="equity-chart__grid" />
          <polyline className="equity-chart__line" points={plot} />
          {points.map((point, index) => {
            const [x, y] = plot.split(" ")[index].split(",");
            return <circle className="equity-chart__point" cx={x} cy={y} key={point.timestamp} r="2"><title>{`${point.timestamp.slice(0, 10)} · equity ${usd.format(point.equityUsd)} · cash ${usd.format(point.cashUsd)} · quantity ${point.quantity} · drawdown ${point.drawdownPct.toFixed(2)}%`}</title></circle>;
          })}
        </svg>
      </div>
      <div className="equity-chart-labels"><span>{points[0].timestamp.slice(0, 10)}</span><span>{points[points.length - 1].timestamp.slice(0, 10)}</span></div>
    </section>
  );
}
