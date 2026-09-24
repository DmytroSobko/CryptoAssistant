import type { AssetPosition, AssetSymbol } from "../types/api";

interface AssetCardProps {
  asset: AssetSymbol;
  position?: AssetPosition;
}

export function AssetCard({ asset, position }: AssetCardProps) {
  const quantity = position?.quantity ?? 0;
  const entry = position?.averageEntryPrice ?? 0;

  return (
    <article className="asset-card">
      <div className="asset-card__heading">
        <h2>{asset}</h2>
        <span className="badge badge--muted">Awaiting market data</span>
      </div>
      <dl className="metrics">
        <div><dt>Price</dt><dd>—</dd></div>
        <div><dt>Trend</dt><dd>—</dd></div>
        <div><dt>Position</dt><dd>{quantity} {asset}</dd></div>
        <div><dt>Average entry</dt><dd>{entry ? `$${entry.toLocaleString()}` : "—"}</dd></div>
        <div><dt>P/L</dt><dd>—</dd></div>
        <div><dt>Local high</dt><dd>—</dd></div>
        <div><dt>Drawdown</dt><dd>—</dd></div>
      </dl>
      <section className="signal">
        <span className="eyebrow">Current action</span>
        <strong>WAIT</strong>
        <p>Market data and the deterministic strategy engine have not been enabled yet.</p>
        <p><span className="muted">Next trigger:</span> Load completed daily candles.</p>
      </section>
    </article>
  );
}

