import { ChangeEvent, FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../api/client";
import { BacktestEquityChart } from "../components/BacktestEquityChart";
import { BacktestSummary } from "../components/BacktestSummary";
import { BacktestTradeTable } from "../components/BacktestTradeTable";
import type { AssetStrategySettings, AssetSymbol, BacktestCandleSet, BacktestResult, BacktestRunSummary, StrategyConfig, StrategyID } from "../types/api";

const assets: AssetSymbol[] = ["BTC", "ETH"];
type SharedNumericField = Exclude<keyof StrategyConfig, "trendMode" | "strategyId" | "entry2DipFromFirstPct" | "entry3DipFromFirstPct" | "estimatedSellFeeBps" | "breakEvenExitFloorEnabled">;
type DipNumericField = "entry2DipFromFirstPct" | "entry3DipFromFirstPct";
type EditableNumericField = SharedNumericField | DipNumericField;

const sharedFields: Array<{ field: SharedNumericField; label: string; step?: number }> = [
  { field: "pullbackMinPct", label: "Pullback minimum (%)" }, { field: "pivotLeft", label: "Pivot left", step: 1 }, { field: "pivotRight", label: "Pivot right", step: 1 }, { field: "breakoutBufferPct", label: "Breakout buffer (%)" },
  { field: "entry1Pct", label: "Entry 1 (%)" }, { field: "entry2Pct", label: "Entry 2 (%)" }, { field: "entry3Pct", label: "Entry 3 (%)" },
  { field: "profitTrigger1Pct", label: "Profit trigger 1 (%)" }, { field: "profitTrigger2Pct", label: "Profit trigger 2 (%)" }, { field: "profitTakePct", label: "Profit take (%)" },
  { field: "drawdown1Pct", label: "Drawdown 1 (%)" }, { field: "drawdown1SellPct", label: "Drawdown 1 sell (%)" }, { field: "drawdown2Pct", label: "Drawdown 2 (%)" }, { field: "drawdown2SellPct", label: "Drawdown 2 sell (%)" }, { field: "drawdown3Pct", label: "Drawdown 3 (%)" }, { field: "drawdown3SellPct", label: "Drawdown 3 sell (%)" },
];
const dipFields: Array<{ field: DipNumericField; label: string }> = [
  { field: "entry2DipFromFirstPct", label: "Entry 2 below Entry 1 (%)" },
  { field: "entry3DipFromFirstPct", label: "Entry 3 below Entry 1 (%)" },
];

function dateValue(timestamp: string) { return timestamp.slice(0, 10); }
function warmupStart(set: BacktestCandleSet) { const date = new Date(`${dateValue(set.firstTimestamp)}T00:00:00Z`); date.setUTCDate(date.getUTCDate() + 199); return date.toISOString().slice(0, 10); }
function displayError(error: unknown) { return error instanceof Error ? error.message : "The local backtest service could not complete that request."; }
function strategyID(config: StrategyConfig): StrategyID { return config.strategyId ?? "RECOVERY_BREAKOUT"; }
function strategyName(id: StrategyID | undefined): string { return id === "DIP_ACCUMULATION" ? "Dip Accumulation" : "Recovery Breakout"; }
function configForStrategy(settings: AssetStrategySettings, id: StrategyID): StrategyConfig { return id === "DIP_ACCUMULATION" ? settings.dipAccumulation : settings.recoveryBreakout; }

function validateConfig(config: StrategyConfig): string | null {
  if (sharedFields.some(({ field }) => !Number.isFinite(config[field]))) return "Every strategy setting must be a finite number.";
  if (!Number.isInteger(config.pivotLeft) || !Number.isInteger(config.pivotRight) || config.pivotLeft < 1 || config.pivotRight < 1) return "Pivot windows must be whole numbers of at least 1.";
  const positive = [config.pullbackMinPct, config.entry1Pct, config.entry2Pct, config.entry3Pct, config.profitTrigger1Pct, config.profitTrigger2Pct, config.profitTakePct, config.drawdown1SellPct, config.drawdown2SellPct, config.drawdown3SellPct];
  if (positive.some((value) => value <= 0 || value > 100) || config.breakoutBufferPct < 0 || config.breakoutBufferPct > 100) return "Configured percentages must be within their supported ranges.";
  if (config.entry1Pct + config.entry2Pct + config.entry3Pct > 100) return "Entry percentages cannot total more than 100%.";
  if (config.profitTrigger2Pct < config.profitTrigger1Pct) return "Profit trigger 2 must be at or above profit trigger 1.";
  if (!(config.drawdown1Pct < 0 && config.drawdown1Pct >= -100 && config.drawdown2Pct < config.drawdown1Pct && config.drawdown2Pct >= -100 && config.drawdown3Pct < config.drawdown2Pct && config.drawdown3Pct >= -100)) return "Drawdown thresholds must become progressively deeper between -100% and 0%.";
  if (strategyID(config) === "DIP_ACCUMULATION") {
    const entry2Dip = config.entry2DipFromFirstPct;
    const entry3Dip = config.entry3DipFromFirstPct;
    if (!Number.isFinite(entry2Dip) || !Number.isFinite(entry3Dip) || entry2Dip! <= 0 || entry3Dip! <= entry2Dip! || entry3Dip! >= 100) return "Entry 3 must be a deeper dip than Entry 2, and both must be between 0 and 100%.";
    if (!config.breakEvenExitFloorEnabled) return "Dip Accumulation requires the break-even exit floor.";
  }
  return null;
}

function validateRun(config: StrategyConfig | null, candleSet: BacktestCandleSet | undefined, start: string, end: string, cash: number, feeBps: number, slippageBps: number): string | null {
  if (!candleSet) return "Import or select a historical candle set first.";
  if (!config) return "Strategy settings are still loading.";
  if (!start || !end || start > end) return "Choose an ordered start and end date.";
  if (start < warmupStart(candleSet)) return "The start date must leave 200 completed daily candles for warm-up.";
  if (end > dateValue(candleSet.lastTimestamp)) return "The end date must be within the selected candle set.";
  if (!Number.isFinite(cash) || cash <= 0) return "Starting cash must be greater than zero.";
  if (!Number.isFinite(feeBps) || feeBps < 0 || !Number.isFinite(slippageBps) || slippageBps < 0) return "Fees and slippage must be non-negative.";
  return validateConfig(config);
}

export function BacktestPage() {
  const [asset, setAsset] = useState<AssetSymbol>("BTC");
  const [profiles, setProfiles] = useState<Record<AssetSymbol, AssetStrategySettings | null>>({ BTC: null, ETH: null });
  const [backtestStrategies, setBacktestStrategies] = useState<Record<AssetSymbol, StrategyID>>({ BTC: "RECOVERY_BREAKOUT", ETH: "RECOVERY_BREAKOUT" });
  const [candleSets, setCandleSets] = useState<BacktestCandleSet[]>([]);
  const [selectedSetID, setSelectedSetID] = useState("");
  const [config, setConfig] = useState<StrategyConfig | null>(null);
  const [sourceLabel, setSourceLabel] = useState("");
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [start, setStart] = useState("");
  const [end, setEnd] = useState("");
  const [startingCash, setStartingCash] = useState(10000);
  const [feeBps, setFeeBps] = useState(10);
  const [slippageBps, setSlippageBps] = useState(5);
  const [result, setResult] = useState<BacktestResult | null>(null);
  const [runs, setRuns] = useState<BacktestRunSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isImporting, setIsImporting] = useState(false);
  const [isRunning, setIsRunning] = useState(false);
  const [isLoadingRun, setIsLoadingRun] = useState(false);
  const fileInput = useRef<HTMLInputElement>(null);
  const selectedSet = useMemo(() => candleSets.find((set) => set.id === selectedSetID), [candleSets, selectedSetID]);
  const isDipAccumulation = config ? strategyID(config) === "DIP_ACCUMULATION" : false;

  useEffect(() => {
    let active = true;
    void Promise.all([api.strategySettings(), api.backtests()]).then(([loadedProfiles, loadedRuns]) => {
      if (!active) return;
      setProfiles({ BTC: loadedProfiles.BTC, ETH: loadedProfiles.ETH });
      setBacktestStrategies({ BTC: loadedProfiles.BTC.selectedStrategyId, ETH: loadedProfiles.ETH.selectedStrategyId });
      setConfig({ ...configForStrategy(loadedProfiles.BTC, loadedProfiles.BTC.selectedStrategyId) });
      setRuns(loadedRuns);
      setIsLoading(false);
    }, (loadError: unknown) => { if (active) { setError(displayError(loadError)); setIsLoading(false); } });
    return () => { active = false; };
  }, []);

  useEffect(() => {
    let active = true;
    setSelectedSetID(""); setStart(""); setEnd(""); setResult(null); setError(null); setNotice(null);
    if (profiles[asset]) setConfig({ ...configForStrategy(profiles[asset]!, backtestStrategies[asset]) });
    void api.backtestCandleSets(asset).then((sets) => {
      if (!active) return;
      setCandleSets(sets);
      if (sets[0]) selectCandleSet(sets[0]);
    }, (loadError: unknown) => { if (active) setError(displayError(loadError)); });
    return () => { active = false; };
    // The run-local copy changes only when the selected asset changes.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [asset]);

  function selectCandleSet(set: BacktestCandleSet) { setSelectedSetID(set.id); setStart(warmupStart(set)); setEnd(dateValue(set.lastTimestamp)); }
  function selectAsset(candidate: AssetSymbol) { setAsset(candidate); }
  function selectBacktestStrategy(candidate: StrategyID) {
    const profile = profiles[asset];
    if (!profile) return;
    setBacktestStrategies((current) => ({ ...current, [asset]: candidate }));
    setConfig({ ...configForStrategy(profile, candidate) });
    setResult(null);
    setError(null);
    setNotice(null);
  }

  async function importCSV(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(null); setNotice(null);
    if (!selectedFile) { setError("Choose a CSV file to import."); return; }
    if (!sourceLabel.trim()) { setError("Add a source label so this data remains auditable."); return; }
    setIsImporting(true);
    try {
      const set = await api.importBacktestCandleSet({ asset, sourceLabel: sourceLabel.trim(), sourceFilename: selectedFile.name, csv: await selectedFile.text() });
      setCandleSets((current) => [set, ...current]); selectCandleSet(set); setSelectedFile(null); setSourceLabel("");
      if (fileInput.current) fileInput.current.value = "";
      setNotice(`${set.sourceFilename} imported with ${set.candleCount.toLocaleString()} daily candles.`);
    } catch (importError) { setError(displayError(importError)); } finally { setIsImporting(false); }
  }

  async function runBacktest(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setError(null); setNotice(null);
    const validation = validateRun(config, selectedSet, start, end, startingCash, feeBps, slippageBps);
    if (validation || !config || !selectedSet) { setError(validation ?? "Backtest settings are incomplete."); return; }
    setIsRunning(true);
    try {
      const run = await api.createBacktest({ candleSetId: selectedSet.id, asset, start: `${start}T00:00:00Z`, end: `${end}T00:00:00Z`, startingCashUsd: startingCash, strategyConfig: config, feeBps, slippageBps, executionModel: "NEXT_DAILY_OPEN" });
      setResult(run.result);
      setRuns((current) => [{ id: run.id, candleSetId: run.candleSetId, asset: run.result.request.asset, start: run.result.request.start, end: run.result.request.end, startingCashUsd: run.result.summary.startingCashUsd, returnPct: run.result.summary.returnPct, maximumDrawdownPct: run.result.summary.maximumDrawdownPct, strategyId: strategyID(run.result.request.strategyConfig), createdAt: run.createdAt, completedAt: run.completedAt }, ...current]);
      setNotice("Backtest completed and saved locally. Live advisory data was not changed.");
    } catch (runError) { setError(displayError(runError)); } finally { setIsRunning(false); }
  }

  async function loadRun(id: string) {
    setError(null); setNotice(null); setIsLoadingRun(true);
    try { const run = await api.backtest(id); setResult(run.result); } catch (loadError) { setError(displayError(loadError)); } finally { setIsLoadingRun(false); }
  }

  function updateNumeric(field: EditableNumericField, event: ChangeEvent<HTMLInputElement>) {
    const value = Number(event.target.value);
    setConfig((current) => current ? { ...current, [field]: value } : current);
  }

  if (isLoading) return <section className="placeholder"><h1>Backtests</h1><p>Loading locally saved strategy profiles and historical data sets.</p></section>;
  return <section className="backtest-page">
    <header className="page-header"><div><p className="eyebrow">Separate hypothetical module</p><h1>Backtests</h1></div><p className="page-note">Historical simulations never alter your live portfolio, advisory history, or strategy state.</p></header>
    <section className="backtest-disclosure"><strong>Hypothetical simulation using completed UTC daily candles.</strong><span>Not investment advice. Past results do not predict future results. Fills use the next daily open, include fees and slippage, and exclude taxes.</span></section>
    <div aria-label="Backtest asset" className="asset-tabs" role="tablist">{assets.map((candidate) => <button aria-selected={asset === candidate} className={asset === candidate ? "asset-tab asset-tab--active" : "asset-tab"} key={candidate} onClick={() => selectAsset(candidate)} role="tab" type="button">{candidate}</button>)}</div>
    <section className="backtest-strategy-picker"><label className="field"><span>Strategy for this simulation</span><select aria-label="Strategy for this simulation" onChange={(event) => selectBacktestStrategy(event.target.value as StrategyID)} value={backtestStrategies[asset]}><option value="RECOVERY_BREAKOUT">Recovery Breakout</option><option value="DIP_ACCUMULATION">Dip Accumulation</option></select></label><p><strong>Live {asset} assistance:</strong> {profiles[asset] ? strategyName(profiles[asset]!.selectedStrategyId) : "Loading"}. Changing this selector only changes the run-local snapshot.</p></section>
    {error && <p className="form-message form-message--error" role="alert">{error}</p>}
    {notice && <p className="form-message form-message--success" role="status">{notice}</p>}

    <section className="backtest-panel"><div className="backtest-section-heading"><div><p className="eyebrow">Immutable source data</p><h2>Historical candles</h2></div></div>
      <form className="backtest-import" onSubmit={importCSV}><label className="field"><span>Source label</span><input onChange={(event) => setSourceLabel(event.target.value)} placeholder="Coinbase daily export" value={sourceLabel} /></label><label className="field"><span>CSV file</span><input accept=".csv,text/csv" onChange={(event) => setSelectedFile(event.target.files?.[0] ?? null)} ref={fileInput} type="file" /></label><button className="secondary-button" disabled={isImporting} type="submit">{isImporting ? "Importing…" : "Import CSV"}</button></form>
      <p className="backtest-help">Required schema: <code>timestamp,open,high,low,close,volume</code>. Timestamps must be UTC midnight; 201 or more contiguous daily candles are required.</p>
      {candleSets.length === 0 ? <p className="muted">No {asset} candle sets have been imported yet.</p> : <div className="candle-set-list">{candleSets.map((set) => <button className={set.id === selectedSetID ? "candle-set candle-set--active" : "candle-set"} key={set.id} onClick={() => selectCandleSet(set)} type="button"><strong>{set.sourceFilename}</strong><span>{set.sourceLabel} · {set.candleCount.toLocaleString()} candles</span><span>{dateValue(set.firstTimestamp)} to {dateValue(set.lastTimestamp)} · SHA {set.originalSha256.slice(0, 12)}</span></button>)}</div>}
    </section>

    <form className="backtest-run-form" onSubmit={runBacktest}><section className="backtest-panel"><div className="backtest-section-heading"><div><p className="eyebrow">Run-local settings</p><h2>Simulation setup</h2></div><p className="muted">Copied from the selected {asset} strategy; edits here do not save to the live strategy.</p></div>
      <div className="backtest-form-grid"><label className="field"><span>Start date</span><input disabled={!selectedSet} max={selectedSet ? dateValue(selectedSet.lastTimestamp) : undefined} min={selectedSet ? warmupStart(selectedSet) : undefined} onChange={(event) => setStart(event.target.value)} type="date" value={start} /></label><label className="field"><span>End date</span><input disabled={!selectedSet} max={selectedSet ? dateValue(selectedSet.lastTimestamp) : undefined} min={start || undefined} onChange={(event) => setEnd(event.target.value)} type="date" value={end} /></label><label className="field"><span>Starting cash (USD)</span><input min="0" onChange={(event) => setStartingCash(Number(event.target.value))} step="any" type="number" value={startingCash} /></label><label className="field"><span>Fees (bps per fill)</span><input min="0" onChange={(event) => setFeeBps(Number(event.target.value))} step="any" type="number" value={feeBps} /></label><label className="field"><span>Slippage (bps per fill)</span><input min="0" onChange={(event) => setSlippageBps(Number(event.target.value))} step="any" type="number" value={slippageBps} /></label><label className="field"><span>Execution model</span><input disabled value="NEXT DAILY OPEN" /></label></div>
      <details className="backtest-config"><summary>Strategy configuration snapshot — {config ? strategyName(strategyID(config)) : "Loading"}</summary>{config && <><div className="backtest-config-grid"><label className="field"><span>Trend mode</span><select onChange={(event) => setConfig((current) => current ? { ...current, trendMode: event.target.value as StrategyConfig["trendMode"] } : current)} value={config.trendMode}><option value="STRICT">STRICT</option><option value="RECOVERY">RECOVERY</option><option value="OFF">OFF</option></select></label>{sharedFields.map(({ field, label, step }) => <label className="field" key={field}><span>{label}</span><input onChange={(event) => updateNumeric(field, event)} step={step ?? "any"} type="number" value={config[field]} /></label>)}{isDipAccumulation && dipFields.map(({ field, label }) => <label className="field" key={field}><span>{label}</span><input min="0" onChange={(event) => updateNumeric(field, event)} step="any" type="number" value={config[field] ?? ""} /></label>)}</div>{isDipAccumulation && <p className="backtest-strategy-note">Dip Accumulation uses the actual simulated Entry 1 fill as its fixed reference. The fees above determine the simulated break-even floor at the next daily open; a protected drawdown sale can be rejected when its net proceeds are below the remaining average entry price.</p>}</>}</details>
      <div className="form-actions"><button className="primary-button" disabled={isRunning || !selectedSet || !config} type="submit">{isRunning ? "Running backtest…" : "Run backtest"}</button></div>
    </section></form>

    {result && <><BacktestSummary asset={result.request.asset} result={result} /><BacktestEquityChart points={result.equityCurve} /><BacktestTradeTable signals={result.signals} trades={result.trades} /></>}
    <section className="backtest-results-section"><div className="backtest-section-heading"><div><p className="eyebrow">Read-only archive</p><h2>Saved runs</h2></div></div>{runs.length === 0 ? <p className="muted">Completed backtests will remain available here.</p> : <div className="saved-runs">{runs.map((run) => <div className="saved-run" key={run.id}><div><strong>{run.asset} · {strategyName(run.strategyId)} · {run.start.slice(0, 10)} to {run.end.slice(0, 10)}</strong><span>Created {new Date(run.createdAt).toLocaleDateString()} · ${run.startingCashUsd.toLocaleString()} starting capital</span></div><div><span className={run.returnPct >= 0 ? "metric--positive" : "metric--negative"}>{run.returnPct >= 0 ? "+" : ""}{run.returnPct.toFixed(2)}%</span><small>max DD {run.maximumDrawdownPct.toFixed(2)}%</small></div><button className="secondary-button" disabled={isLoadingRun} onClick={() => void loadRun(run.id)} type="button">{isLoadingRun ? "Loading…" : "View"}</button></div>)}</div>}</section>
  </section>;
}
