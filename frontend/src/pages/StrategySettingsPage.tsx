import { FormEvent, useEffect, useState } from "react";
import { api } from "../api/client";
import type { AssetSymbol, StrategyConfig } from "../types/api";

type NumericField = Exclude<keyof StrategyConfig, "trendMode">;
type ConfigForm = { [Field in NumericField]: string } & Pick<StrategyConfig, "trendMode">;

interface FieldDefinition {
  field: NumericField;
  label: string;
  min?: number;
  step?: number;
}

const sections: Array<{ title: string; fields: FieldDefinition[] }> = [
  {
    title: "Correction and trend",
    fields: [
      { field: "pullbackMinPct", label: "Minimum pullback (%)", min: 0 },
      { field: "pivotLeft", label: "Pivot candles left", min: 1, step: 1 },
      { field: "pivotRight", label: "Pivot candles right", min: 1, step: 1 },
      { field: "breakoutBufferPct", label: "Breakout buffer (%)", min: 0 },
    ],
  },
  {
    title: "Staged entry",
    fields: [
      { field: "entry1Pct", label: "Entry 1 (%)", min: 0 },
      { field: "entry2Pct", label: "Entry 2 (%)", min: 0 },
      { field: "entry3Pct", label: "Entry 3 (%)", min: 0 },
    ],
  },
  {
    title: "Profit protection",
    fields: [
      { field: "profitTrigger1Pct", label: "Profit trigger 1 (%)", min: 0 },
      { field: "profitTrigger2Pct", label: "Profit trigger 2 (%)", min: 0 },
      { field: "profitTakePct", label: "Profit take (%)", min: 0 },
    ],
  },
  {
    title: "Drawdown exits",
    fields: [
      { field: "drawdown1Pct", label: "Drawdown 1 (%)" },
      { field: "drawdown1SellPct", label: "Drawdown 1 sell (%)", min: 0 },
      { field: "drawdown2Pct", label: "Drawdown 2 (%)" },
      { field: "drawdown2SellPct", label: "Drawdown 2 sell (%)", min: 0 },
      { field: "drawdown3Pct", label: "Drawdown 3 (%)" },
      { field: "drawdown3SellPct", label: "Drawdown 3 sell (%)", min: 0 },
    ],
  },
];

function formFromConfig(config: StrategyConfig): ConfigForm {
  return {
    pullbackMinPct: String(config.pullbackMinPct), pivotLeft: String(config.pivotLeft), pivotRight: String(config.pivotRight),
    breakoutBufferPct: String(config.breakoutBufferPct), trendMode: config.trendMode,
    entry1Pct: String(config.entry1Pct), entry2Pct: String(config.entry2Pct), entry3Pct: String(config.entry3Pct),
    profitTrigger1Pct: String(config.profitTrigger1Pct), profitTrigger2Pct: String(config.profitTrigger2Pct), profitTakePct: String(config.profitTakePct),
    drawdown1Pct: String(config.drawdown1Pct), drawdown1SellPct: String(config.drawdown1SellPct),
    drawdown2Pct: String(config.drawdown2Pct), drawdown2SellPct: String(config.drawdown2SellPct),
    drawdown3Pct: String(config.drawdown3Pct), drawdown3SellPct: String(config.drawdown3SellPct),
  };
}

function numberValue(value: string, label: string): number {
  const number = Number(value);
  if (value.trim() === "" || !Number.isFinite(number)) throw new Error(`${label} must be a finite number.`);
  return number;
}

function configFromForm(form: ConfigForm): StrategyConfig {
  const config: StrategyConfig = {
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
  return config;
}

export function StrategySettingsPage() {
  const [selectedAsset, setSelectedAsset] = useState<AssetSymbol>("BTC");
  const [forms, setForms] = useState<Record<AssetSymbol, ConfigForm | null>>({ BTC: null, ETH: null });
  const [error, setError] = useState<string | null>(null);
  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    let active = true;
    void api.config().then(
      (configs) => {
        if (!active) return;
        setForms({ BTC: formFromConfig(configs.BTC), ETH: formFromConfig(configs.ETH) });
        setIsLoading(false);
      },
      (loadError: unknown) => {
        if (!active) return;
        setError(loadError instanceof Error ? loadError.message : "Could not load strategy settings.");
        setIsLoading(false);
      },
    );
    return () => { active = false; };
  }, []);

  const form = forms[selectedAsset];
  const updateField = (field: NumericField, value: string) => {
    setForms((current) => ({ ...current, [selectedAsset]: current[selectedAsset] ? { ...current[selectedAsset], [field]: value } : null }));
  };

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!form) return;
    setError(null);
    setSaved(false);

    let config: StrategyConfig;
    try {
      config = configFromForm(form);
    } catch (validationError) {
      setError(validationError instanceof Error ? validationError.message : "Check the strategy settings.");
      return;
    }

    setIsSaving(true);
    try {
      await api.saveConfig(selectedAsset, config);
      setSaved(true);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Could not save strategy settings.");
    } finally {
      setIsSaving(false);
    }
  }

  if (isLoading) return <section className="placeholder"><h1>Strategy settings</h1><p>Loading the locally stored BTC and ETH rules.</p></section>;
  if (!form) return <section className="placeholder"><h1>Strategy settings</h1><p className="form-message form-message--error">{error ?? "Strategy settings are unavailable."}</p></section>;

  return (
    <section className="settings-page">
      <header className="page-header">
        <div>
          <p className="eyebrow">Deterministic rules</p>
          <h1>Strategy settings</h1>
        </div>
        <p className="page-note">Changes affect future advisory evaluations only. They never execute a trade.</p>
      </header>

      <div aria-label="Asset settings" className="asset-tabs" role="tablist">
        {(["BTC", "ETH"] as AssetSymbol[]).map((asset) => <button aria-selected={selectedAsset === asset} className={selectedAsset === asset ? "asset-tab asset-tab--active" : "asset-tab" key={asset} onClick={() => { setSelectedAsset(asset); setError(null); setSaved(false); }} role="tab" type="button">{asset}</button>)}
      </div>

      <form className="settings-form" onSubmit={save}>
        <section className="settings-section">
          <h2>Trend filter</h2>
          <label className="field">
            <span>Mode</span>
            <select aria-label="Trend filter mode" onChange={(event) => setForms((current) => ({ ...current, [selectedAsset]: current[selectedAsset] ? { ...current[selectedAsset], trendMode: event.target.value as StrategyConfig["trendMode"] } : null }))} value={form.trendMode}>
              <option value="STRICT">STRICT — close must be above SMA200</option>
              <option value="RECOVERY">RECOVERY — allow a confirmed recovery below SMA200</option>
              <option value="OFF">OFF — do not use the SMA200 filter</option>
            </select>
          </label>
        </section>
        <div className="settings-grid">
          {sections.map((section) => (
            <section className="settings-section" key={section.title}>
              <h2>{section.title}</h2>
              <div className="settings-fields">
                {section.fields.map(({ field, label, min, step }) => <label className="field" key={field}><span>{label}</span><input aria-label={label} inputMode="decimal" min={min} onChange={(event) => updateField(field, event.target.value)} step={step ?? "any"} type="number" value={form[field]} /></label>)}
              </div>
            </section>
          ))}
        </div>
        {error && <p className="form-message form-message--error" role="alert">{error}</p>}
        {saved && <p className="form-message form-message--success" role="status">{selectedAsset} strategy settings saved locally.</p>}
        <div className="form-actions"><button className="primary-button" disabled={isSaving} type="submit">{isSaving ? "Saving…" : `Save ${selectedAsset} settings`}</button></div>
      </form>
    </section>
  );
}
