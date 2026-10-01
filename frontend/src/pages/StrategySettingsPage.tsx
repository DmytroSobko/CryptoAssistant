import { FormEvent, useEffect, useState } from "react";
import { api } from "../api/client";
import type { AssetStrategySettings, AssetSymbol, StrategyConfig, StrategyID } from "../types/api";

type NumericField =
  | "pullbackMinPct" | "pivotLeft" | "pivotRight" | "breakoutBufferPct"
  | "entry1Pct" | "entry2Pct" | "entry3Pct"
  | "profitTrigger1Pct" | "profitTrigger2Pct" | "profitTakePct"
  | "drawdown1Pct" | "drawdown1SellPct" | "drawdown2Pct" | "drawdown2SellPct" | "drawdown3Pct" | "drawdown3SellPct"
  | "entry2DipFromFirstPct" | "entry3DipFromFirstPct" | "estimatedSellFeeBps";

type ConfigForm = Record<NumericField, string> & {
  trendMode: StrategyConfig["trendMode"];
  breakEvenExitFloorEnabled: boolean;
};

interface ProfileForm {
  selectedStrategyId: StrategyID;
  recoveryBreakout: ConfigForm;
  dipAccumulation: ConfigForm;
}

interface FieldDefinition {
  field: NumericField;
  label: string;
  min?: number;
  step?: number;
}

const initialEntryFields: FieldDefinition[] = [
  { field: "pullbackMinPct", label: "Minimum pullback (%)", min: 0 },
  { field: "pivotLeft", label: "Pivot candles left", min: 1, step: 1 },
  { field: "pivotRight", label: "Pivot candles right", min: 1, step: 1 },
  { field: "breakoutBufferPct", label: "Breakout buffer (%)", min: 0 },
];
const allocationFields: FieldDefinition[] = [
  { field: "entry1Pct", label: "Entry 1 (%)", min: 0 },
  { field: "entry2Pct", label: "Entry 2 (%)", min: 0 },
  { field: "entry3Pct", label: "Entry 3 (%)", min: 0 },
];
const profitFields: FieldDefinition[] = [
  { field: "profitTrigger1Pct", label: "Profit trigger 1 (%)", min: 0 },
  { field: "profitTrigger2Pct", label: "Profit trigger 2 (%)", min: 0 },
  { field: "profitTakePct", label: "Profit take (%)", min: 0 },
];
const drawdownFields: FieldDefinition[] = [
  { field: "drawdown1Pct", label: "Drawdown 1 (%)" },
  { field: "drawdown1SellPct", label: "Drawdown 1 sell (%)", min: 0 },
  { field: "drawdown2Pct", label: "Drawdown 2 (%)" },
  { field: "drawdown2SellPct", label: "Drawdown 2 sell (%)", min: 0 },
  { field: "drawdown3Pct", label: "Drawdown 3 (%)" },
  { field: "drawdown3SellPct", label: "Drawdown 3 sell (%)", min: 0 },
];
const dipFields: FieldDefinition[] = [
  { field: "entry2DipFromFirstPct", label: "Entry 2 below Entry 1 (%)", min: 0 },
  { field: "entry3DipFromFirstPct", label: "Entry 3 below Entry 1 (%)", min: 0 },
];

function strategyName(strategyId: StrategyID): string {
  return strategyId === "DIP_ACCUMULATION" ? "Dip Accumulation" : "Recovery Breakout";
}

function selectedConfigKey(strategyId: StrategyID): "recoveryBreakout" | "dipAccumulation" {
  return strategyId === "DIP_ACCUMULATION" ? "dipAccumulation" : "recoveryBreakout";
}

function formFromConfig(config: StrategyConfig): ConfigForm {
  return {
    pullbackMinPct: String(config.pullbackMinPct), pivotLeft: String(config.pivotLeft), pivotRight: String(config.pivotRight),
    breakoutBufferPct: String(config.breakoutBufferPct), trendMode: config.trendMode,
    entry1Pct: String(config.entry1Pct), entry2Pct: String(config.entry2Pct), entry3Pct: String(config.entry3Pct),
    profitTrigger1Pct: String(config.profitTrigger1Pct), profitTrigger2Pct: String(config.profitTrigger2Pct), profitTakePct: String(config.profitTakePct),
    drawdown1Pct: String(config.drawdown1Pct), drawdown1SellPct: String(config.drawdown1SellPct),
    drawdown2Pct: String(config.drawdown2Pct), drawdown2SellPct: String(config.drawdown2SellPct),
    drawdown3Pct: String(config.drawdown3Pct), drawdown3SellPct: String(config.drawdown3SellPct),
    entry2DipFromFirstPct: String(config.entry2DipFromFirstPct ?? 10), entry3DipFromFirstPct: String(config.entry3DipFromFirstPct ?? 20),
    estimatedSellFeeBps: String(config.estimatedSellFeeBps ?? 10), breakEvenExitFloorEnabled: config.breakEvenExitFloorEnabled ?? true,
  };
}

function formFromSettings(settings: AssetStrategySettings): ProfileForm {
  return {
    selectedStrategyId: settings.selectedStrategyId,
    recoveryBreakout: formFromConfig(settings.recoveryBreakout),
    dipAccumulation: formFromConfig(settings.dipAccumulation),
  };
}

function numberValue(value: string, label: string): number {
  const number = Number(value);
  if (value.trim() === "" || !Number.isFinite(number)) throw new Error(`${label} must be a finite number.`);
  return number;
}

function configFromForm(form: ConfigForm, strategyId: StrategyID): StrategyConfig {
  const config: StrategyConfig = {
    strategyId,
    pullbackMinPct: numberValue(form.pullbackMinPct, "Minimum pullback"),
    pivotLeft: numberValue(form.pivotLeft, "Pivot candles left"),
    pivotRight: numberValue(form.pivotRight, "Pivot candles right"),
    breakoutBufferPct: numberValue(form.breakoutBufferPct, "Breakout buffer"),
    trendMode: form.trendMode,
    entry1Pct: numberValue(form.entry1Pct, "Entry 1"), entry2Pct: numberValue(form.entry2Pct, "Entry 2"), entry3Pct: numberValue(form.entry3Pct, "Entry 3"),
    profitTrigger1Pct: numberValue(form.profitTrigger1Pct, "Profit trigger 1"), profitTrigger2Pct: numberValue(form.profitTrigger2Pct, "Profit trigger 2"), profitTakePct: numberValue(form.profitTakePct, "Profit take"),
    drawdown1Pct: numberValue(form.drawdown1Pct, "Drawdown 1"), drawdown1SellPct: numberValue(form.drawdown1SellPct, "Drawdown 1 sell"),
    drawdown2Pct: numberValue(form.drawdown2Pct, "Drawdown 2"), drawdown2SellPct: numberValue(form.drawdown2SellPct, "Drawdown 2 sell"),
    drawdown3Pct: numberValue(form.drawdown3Pct, "Drawdown 3"), drawdown3SellPct: numberValue(form.drawdown3SellPct, "Drawdown 3 sell"),
  };
  const positivePercentages = [config.pullbackMinPct, config.entry1Pct, config.entry2Pct, config.entry3Pct, config.profitTrigger1Pct, config.profitTrigger2Pct, config.profitTakePct, config.drawdown1SellPct, config.drawdown2SellPct, config.drawdown3SellPct];
  if (positivePercentages.some((value) => value <= 0 || value > 100)) throw new Error("Positive percentage values must be greater than 0 and at most 100.");
  if (!Number.isInteger(config.pivotLeft) || !Number.isInteger(config.pivotRight) || config.pivotLeft < 1 || config.pivotRight < 1) throw new Error("Pivot windows must both be whole numbers of at least 1.");
  if (config.breakoutBufferPct < 0 || config.breakoutBufferPct > 100) throw new Error("Breakout buffer must be between 0 and 100.");
  if (config.entry1Pct + config.entry2Pct + config.entry3Pct > 100) throw new Error("Entry percentages cannot total more than 100.");
  if (config.profitTrigger2Pct < config.profitTrigger1Pct) throw new Error("Profit trigger 2 must be at or above profit trigger 1.");
  if (config.drawdown1Pct >= 0 || config.drawdown2Pct >= 0 || config.drawdown3Pct >= 0 || config.drawdown1Pct < -100 || config.drawdown2Pct < -100 || config.drawdown3Pct < -100) throw new Error("Drawdown thresholds must be below 0 and no less than -100.");
  if (!(config.drawdown1Pct > config.drawdown2Pct && config.drawdown2Pct > config.drawdown3Pct)) throw new Error("Drawdown thresholds must become progressively deeper.");
  if (strategyId === "DIP_ACCUMULATION") {
    const entry2DipFromFirstPct = numberValue(form.entry2DipFromFirstPct, "Entry 2 dip");
    const entry3DipFromFirstPct = numberValue(form.entry3DipFromFirstPct, "Entry 3 dip");
    const estimatedSellFeeBps = numberValue(form.estimatedSellFeeBps, "Estimated sell fee");
    if (entry2DipFromFirstPct <= 0 || entry2DipFromFirstPct >= 100 || entry3DipFromFirstPct <= entry2DipFromFirstPct || entry3DipFromFirstPct >= 100) throw new Error("Entry 3 must be a deeper dip than Entry 2, and both must be between 0 and 100%.");
    if (estimatedSellFeeBps < 0 || estimatedSellFeeBps >= 10000) throw new Error("Estimated sell fee must be at least 0 and below 10,000 bps.");
    if (!form.breakEvenExitFloorEnabled) throw new Error("Dip Accumulation requires the break-even exit floor.");
    return { ...config, entry2DipFromFirstPct, entry3DipFromFirstPct, estimatedSellFeeBps, breakEvenExitFloorEnabled: true };
  }
  return config;
}

function settingsFromForm(form: ProfileForm): AssetStrategySettings {
  return { selectedStrategyId: form.selectedStrategyId, recoveryBreakout: configFromForm(form.recoveryBreakout, "RECOVERY_BREAKOUT"), dipAccumulation: configFromForm(form.dipAccumulation, "DIP_ACCUMULATION") };
}

function NumberFields({ fields, form, onChange }: { fields: FieldDefinition[]; form: ConfigForm; onChange: (field: NumericField, value: string) => void }) {
  return <div className="settings-fields">{fields.map(({ field, label, min, step }) => <label className="field" key={field}><span>{label}</span><input aria-label={label} inputMode="decimal" min={min} onChange={(event) => onChange(field, event.target.value)} step={step ?? "any"} type="number" value={form[field]} /></label>)}</div>;
}

export function StrategySettingsPage() {
  const [selectedAsset, setSelectedAsset] = useState<AssetSymbol>("BTC");
  const [forms, setForms] = useState<Record<AssetSymbol, ProfileForm | null>>({ BTC: null, ETH: null });
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let active = true;
    void api.strategySettings().then(
      (settings) => { if (active) { setForms({ BTC: formFromSettings(settings.BTC), ETH: formFromSettings(settings.ETH) }); setIsLoading(false); } },
      (loadError: unknown) => { if (active) { setError(loadError instanceof Error ? loadError.message : "Could not load strategy settings."); setIsLoading(false); } },
    );
    return () => { active = false; };
  }, []);

  const profile = forms[selectedAsset];
  const selectedKey = profile ? selectedConfigKey(profile.selectedStrategyId) : "recoveryBreakout";
  const form = profile?.[selectedKey];
  const isDipAccumulation = profile?.selectedStrategyId === "DIP_ACCUMULATION";
  const updateField = (field: NumericField, value: string) => setForms((current) => {
    const currentProfile = current[selectedAsset];
    if (!currentProfile) return current;
    const key = selectedConfigKey(currentProfile.selectedStrategyId);
    return { ...current, [selectedAsset]: { ...currentProfile, [key]: { ...currentProfile[key], [field]: value } } };
  });
  const updateTrendMode = (trendMode: StrategyConfig["trendMode"]) => setForms((current) => {
    const currentProfile = current[selectedAsset];
    if (!currentProfile) return current;
    const key = selectedConfigKey(currentProfile.selectedStrategyId);
    return { ...current, [selectedAsset]: { ...currentProfile, [key]: { ...currentProfile[key], trendMode } } };
  });
  const selectAsset = (asset: AssetSymbol) => { setSelectedAsset(asset); setError(null); setSaved(false); };
  const selectStrategy = (selectedStrategyId: StrategyID) => { setForms((current) => current[selectedAsset] ? { ...current, [selectedAsset]: { ...current[selectedAsset]!, selectedStrategyId } } : current); setError(null); setSaved(false); };

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!profile) return;
    setError(null); setSaved(false);
    let settings: AssetStrategySettings;
    try { settings = settingsFromForm(profile); } catch (validationError) { setError(validationError instanceof Error ? validationError.message : "Check the strategy settings."); return; }
    setIsSaving(true);
    try { await api.saveStrategySettings(selectedAsset, settings); setSaved(true); } catch (saveError) { setError(saveError instanceof Error ? saveError.message : "Could not save strategy settings."); } finally { setIsSaving(false); }
  }

  if (isLoading) return <section className="placeholder"><h1>Strategy settings</h1><p>Loading the locally stored BTC and ETH rules.</p></section>;
  if (!form || !profile) return <section className="placeholder"><h1>Strategy settings</h1><p className="form-message form-message--error">{error ?? "Strategy settings are unavailable."}</p></section>;
  return <section className="settings-page">
    <header className="page-header"><div><p className="eyebrow">Deterministic rules</p><h1>Strategy settings</h1></div><p className="page-note">Changes affect future advisory evaluations only. They never execute a trade.</p></header>
    <div aria-label="Asset settings" className="asset-tabs" role="tablist">{(["BTC", "ETH"] as AssetSymbol[]).map((asset) => <button aria-selected={selectedAsset === asset} className={selectedAsset === asset ? "asset-tab asset-tab--active" : "asset-tab"} key={asset} onClick={() => selectAsset(asset)} role="tab" type="button">{asset}</button>)}</div>
    <form className="settings-form" onSubmit={save}>
      <section className="settings-section strategy-selector"><div><h2>Strategy</h2><p className="settings-help">Choose the single ruleset that produces future {selectedAsset} advisories. Each strategy keeps its own saved settings.</p></div><label className="field"><span>Active strategy for {selectedAsset}</span><select aria-label={`Active strategy for ${selectedAsset}`} onChange={(event) => selectStrategy(event.target.value as StrategyID)} value={profile.selectedStrategyId}><option value="RECOVERY_BREAKOUT">Recovery Breakout</option><option value="DIP_ACCUMULATION">Dip Accumulation</option></select></label><p className="strategy-description">{isDipAccumulation ? "Confirms an initial recovery entry, then averages down at fixed percentages below that first-entry reference. Drawdown exits are held to break-even after estimated fees." : "Uses the existing recovery-breakout rules for each staged entry and preserves the MVP sell behaviour."}</p></section>
      <section className="settings-section"><h2>Initial entry confirmation</h2><p className="settings-help">Both strategies require this correction, recovery, and trend gate before their first entry.</p><label className="field"><span>Trend mode</span><select aria-label="Trend filter mode" onChange={(event) => updateTrendMode(event.target.value as StrategyConfig["trendMode"])} value={form.trendMode}><option value="STRICT">STRICT — close must be above SMA200</option><option value="RECOVERY">RECOVERY — allow a confirmed recovery below SMA200</option><option value="OFF">OFF — do not use the SMA200 filter</option></select></label><NumberFields fields={initialEntryFields} form={form} onChange={updateField} /></section>
      <div className="settings-grid">
        <section className="settings-section"><h2>{isDipAccumulation ? "Average-down entries" : "Staged entry"}</h2><p className="settings-help">{isDipAccumulation ? "Entry 2 and Entry 3 are based on the fixed first-entry signal reference, not another breakout." : "Each allocation requires its own confirmed recovery breakout."}</p><NumberFields fields={allocationFields} form={form} onChange={updateField} />{isDipAccumulation && <NumberFields fields={dipFields} form={form} onChange={updateField} />}</section>
        <section className="settings-section"><h2>Profit protection</h2><NumberFields fields={profitFields} form={form} onChange={updateField} /></section>
        <section className="settings-section"><h2>Drawdown exits</h2><NumberFields fields={drawdownFields} form={form} onChange={updateField} /></section>
        {isDipAccumulation && <section className="settings-section"><h2>Break-even exit floor</h2><p className="settings-help">When a drawdown exit is due, the advisory supplies a minimum gross sale price. It may keep an underwater position open until the expected net proceeds meet the average entry price.</p><label className="field"><span>Estimated sell fee (bps)</span><input aria-label="Estimated sell fee in basis points" min="0" onChange={(event) => updateField("estimatedSellFeeBps", event.target.value)} step="any" type="number" value={form.estimatedSellFeeBps} /></label><label className="checkbox-field"><input checked={form.breakEvenExitFloorEnabled} disabled type="checkbox" /> Break-even exit floor enabled (required for this strategy)</label><p className="settings-disclosure">This is an advisory limit, not a guarantee of a manual market-order fill. Use an appropriate limit price when acting on a protected exit.</p></section>}
      </div>
      {error && <p className="form-message form-message--error" role="alert">{error}</p>}
      {saved && <p className="form-message form-message--success" role="status">{strategyName(profile.selectedStrategyId)} is saved as the active {selectedAsset} strategy.</p>}
      <div className="form-actions"><button className="primary-button" disabled={isSaving} type="submit">{isSaving ? "Saving…" : `Save ${selectedAsset} settings`}</button></div>
    </form>
  </section>;
}
