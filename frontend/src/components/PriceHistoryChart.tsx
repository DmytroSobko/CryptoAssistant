import { useEffect, useMemo, useRef, useState, type FormEvent, type PointerEvent } from "react";
import type { AssetSymbol, MarketPriceHistory, MarketPricePoint } from "../types/api";

const CHART_HEIGHT = 156;
const MIN_RECORD_LIMIT = 1;
const MAX_RECORD_LIMIT = 100000;

interface PriceHistoryChartProps {
  asset: AssetSymbol;
  history: MarketPriceHistory;
  onRecordLimitChange: (asset: AssetSymbol, recordLimit: number) => Promise<void>;
}

interface ChartPoint extends MarketPricePoint {
  time: number;
}

interface HoverPoint extends ChartPoint {
  x: number;
  y: number;
}

function priceLabel(value: number): string {
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: value >= 1000 ? 0 : 2 }).format(value);
}

function timeLabel(value: number): string {
  return new Intl.DateTimeFormat("en-US", { hour: "numeric", minute: "2-digit" }).format(value);
}

function tooltipTime(value: number): string {
  return new Intl.DateTimeFormat("en-US", { dateStyle: "medium", timeStyle: "short" }).format(value);
}

export function PriceHistoryChart({ asset, history, onRecordLimitChange }: PriceHistoryChartProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const renderedPointsRef = useRef<HoverPoint[]>([]);
  const [width, setWidth] = useState(0);
  const [hoverPoint, setHoverPoint] = useState<HoverPoint | null>(null);
  const [recordLimitDraft, setRecordLimitDraft] = useState(String(history.recordLimit));
  const [settingsMessage, setSettingsMessage] = useState<string | null>(null);
  const [settingsError, setSettingsError] = useState(false);
  const [isSaving, setIsSaving] = useState(false);

  const points = useMemo(() => history.points
    .map((point) => ({ ...point, time: new Date(point.timestamp).valueOf() }))
    .filter((point) => Number.isFinite(point.time) && Number.isFinite(point.price) && point.price > 0)
    .sort((left, right) => left.time - right.time), [history.points]);

  useEffect(() => {
    setRecordLimitDraft(String(history.recordLimit));
  }, [history.recordLimit]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const updateWidth = () => setWidth(Math.max(0, Math.floor(container.getBoundingClientRect().width)));
    updateWidth();
    const observer = new ResizeObserver(updateWidth);
    observer.observe(container);
    return () => observer.disconnect();
  }, []);

  useEffect(() => {
    setHoverPoint(null);
  }, [history.points]);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || width <= 0) return;
    const ratio = window.devicePixelRatio || 1;
    canvas.width = Math.round(width * ratio);
    canvas.height = Math.round(CHART_HEIGHT * ratio);
    const context = canvas.getContext("2d");
    if (!context) return;
    context.setTransform(ratio, 0, 0, ratio, 0, 0);
    context.clearRect(0, 0, width, CHART_HEIGHT);

    const margin = { top: 10, right: 8, bottom: 25, left: 54 };
    const plotWidth = Math.max(1, width - margin.left - margin.right);
    const plotHeight = Math.max(1, CHART_HEIGHT - margin.top - margin.bottom);
    const fallbackTime = Date.now();
    const firstTime = points[0]?.time ?? fallbackTime;
    const lastTime = points[points.length - 1]?.time ?? fallbackTime;
    const dataSpan = Math.max(0, lastTime - firstTime);
    const timePadding = dataSpan > 0 ? Math.max(1000, dataSpan * 0.03) : 30_000;
    const startTime = firstTime - timePadding;
    const endTime = lastTime + timePadding;
    const windowMs = Math.max(1, endTime - startTime);
    const visiblePoints = points;
    const priceBounds = visiblePoints.reduce((bounds, point) => ({ min: Math.min(bounds.min, point.price), max: Math.max(bounds.max, point.price) }), { min: Number.POSITIVE_INFINITY, max: Number.NEGATIVE_INFINITY });
    const rawMin = visiblePoints.length > 0 ? priceBounds.min : 0;
    const rawMax = visiblePoints.length > 0 ? priceBounds.max : 1;
    const rawRange = rawMax - rawMin;
    const padding = rawRange > 0 ? rawRange * 0.12 : Math.max(rawMax * 0.001, 1);
    const minPrice = rawMin - padding;
    const maxPrice = rawMax + padding;
    const priceRange = Math.max(1, maxPrice - minPrice);

    context.font = "10px Inter, system-ui, sans-serif";
    context.textBaseline = "middle";
    for (let index = 0; index < 3; index++) {
      const ratioY = index / 2;
      const y = margin.top + plotHeight * ratioY;
      const value = maxPrice - priceRange * ratioY;
      context.strokeStyle = "rgba(142, 160, 182, .14)";
      context.lineWidth = 1;
      context.beginPath();
      context.moveTo(margin.left, y);
      context.lineTo(width - margin.right, y);
      context.stroke();
      context.fillStyle = "#71849b";
      context.textAlign = "right";
      context.fillText(priceLabel(value), margin.left - 7, y);
    }

    context.textBaseline = "alphabetic";
    for (let index = 0; index < 3; index++) {
      const ratioX = index / 2;
      const x = margin.left + plotWidth * ratioX;
      context.fillStyle = "#71849b";
      context.textAlign = index === 0 ? "left" : index === 2 ? "right" : "center";
      context.fillText(timeLabel(startTime + windowMs * ratioX), x, CHART_HEIGHT - 5);
    }

    const maxRenderedPoints = Math.max(200, Math.floor(plotWidth * 2));
    const step = Math.max(1, Math.ceil(visiblePoints.length / maxRenderedPoints));
    const sampled = visiblePoints.filter((_, index) => index % step === 0 || index === visiblePoints.length - 1);
    const rendered = sampled.map((point) => ({
      ...point,
      x: margin.left + ((point.time - startTime) / windowMs) * plotWidth,
      y: margin.top + ((maxPrice - point.price) / priceRange) * plotHeight,
    }));
    renderedPointsRef.current = rendered;

    if (rendered.length > 0) {
      const rising = rendered[rendered.length - 1].price >= rendered[0].price;
      const lineColor = rising ? "#47d7a4" : "#ff8f8f";
      const fillColor = rising ? "rgba(71, 215, 164, 0)" : "rgba(255, 143, 143, 0)";
      const fillStart = rising ? "rgba(71, 215, 164, .18)" : "rgba(255, 143, 143, .17)";
      context.beginPath();
      rendered.forEach((point, index) => index === 0 ? context.moveTo(point.x, point.y) : context.lineTo(point.x, point.y));
      context.strokeStyle = lineColor;
      context.lineWidth = 2;
      context.lineJoin = "round";
      context.lineCap = "round";
      context.stroke();

      if (rendered.length > 1) {
        context.lineTo(rendered[rendered.length - 1].x, margin.top + plotHeight);
        context.lineTo(rendered[0].x, margin.top + plotHeight);
        context.closePath();
        const gradient = context.createLinearGradient(0, margin.top, 0, margin.top + plotHeight);
        gradient.addColorStop(0, fillStart);
        gradient.addColorStop(1, fillColor);
        context.fillStyle = gradient;
        context.fill();
      } else {
        context.beginPath();
        context.arc(rendered[0].x, rendered[0].y, 3, 0, Math.PI * 2);
        context.fillStyle = lineColor;
        context.fill();
      }
    }

    if (hoverPoint) {
      context.strokeStyle = "rgba(220, 231, 243, .45)";
      context.setLineDash([3, 3]);
      context.beginPath();
      context.moveTo(hoverPoint.x, margin.top);
      context.lineTo(hoverPoint.x, margin.top + plotHeight);
      context.stroke();
      context.setLineDash([]);
      context.beginPath();
      context.arc(hoverPoint.x, hoverPoint.y, 3.5, 0, Math.PI * 2);
      context.fillStyle = "#edf3fb";
      context.fill();
    }
  }, [hoverPoint, points, width]);

  function handlePointerMove(event: PointerEvent<HTMLCanvasElement>) {
    const rendered = renderedPointsRef.current;
    if (rendered.length === 0) return;
    const bounds = event.currentTarget.getBoundingClientRect();
    const pointerX = event.clientX - bounds.left;
    let closest = rendered[0];
    for (const point of rendered) {
      if (Math.abs(point.x - pointerX) < Math.abs(closest.x - pointerX)) closest = point;
    }
    setHoverPoint(closest);
  }

  async function saveRecordLimit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const recordLimit = Number(recordLimitDraft);
    if (!Number.isInteger(recordLimit) || recordLimit < MIN_RECORD_LIMIT || recordLimit > MAX_RECORD_LIMIT) {
      setSettingsError(true);
      setSettingsMessage("Enter a whole number from 1 to 100,000.");
      return;
    }
    setIsSaving(true);
    setSettingsMessage(null);
    try {
      await onRecordLimitChange(asset, recordLimit);
      setSettingsError(false);
      setSettingsMessage(`Keeping the latest ${recordLimit.toLocaleString()} samples.`);
    } catch (error) {
      setSettingsError(true);
      setSettingsMessage(error instanceof Error ? error.message : "Could not save the history limit.");
    } finally {
      setIsSaving(false);
    }
  }

  return (
    <section className="price-history">
      <div className="price-history__heading">
        <div><span className="eyebrow">Price trail</span><strong>All retained records</strong></div>
        <span>{points.length.toLocaleString()} sample{points.length === 1 ? "" : "s"}</span>
      </div>
      <div className="price-history__canvas" ref={containerRef}>
        <canvas aria-label={`${asset} price history across all retained records`} height={CHART_HEIGHT} onPointerLeave={() => setHoverPoint(null)} onPointerMove={handlePointerMove} ref={canvasRef} />
        {points.length === 0 && <p className="price-history__empty">Collecting live price samples…</p>}
        {hoverPoint && <div className="price-history__tooltip" style={{ left: Math.min(Math.max(70, hoverPoint.x), Math.max(70, width - 70)), top: Math.max(48, hoverPoint.y - 8) }}><strong>{priceLabel(hoverPoint.price)}</strong><span>{tooltipTime(hoverPoint.time)}</span></div>}
      </div>
      <form className="price-history__settings" onSubmit={(event) => { void saveRecordLimit(event); }}>
        <label htmlFor={`${asset}-history-limit`}>Keep latest</label>
        <input disabled={isSaving} id={`${asset}-history-limit`} inputMode="numeric" max={MAX_RECORD_LIMIT} min={MIN_RECORD_LIMIT} onChange={(event) => { setRecordLimitDraft(event.target.value); setSettingsMessage(null); }} step="1" type="number" value={recordLimitDraft} />
        <span>records</span>
        <button className="secondary-button" disabled={isSaving} type="submit">{isSaving ? "Saving…" : "Apply"}</button>
      </form>
      {settingsMessage && <p className={settingsError ? "price-history__message price-history__message--error" : "price-history__message"}>{settingsMessage}</p>}
    </section>
  );
}
