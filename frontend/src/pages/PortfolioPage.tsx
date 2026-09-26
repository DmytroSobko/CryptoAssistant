import { FormEvent, useEffect, useState } from "react";
import { api } from "../api/client";
import type { AssetSymbol, Portfolio, PortfolioInput } from "../types/api";

type AssetForm = Record<"quantity" | "averageEntryPrice" | "cashAllocated", string>;

interface PortfolioForm {
  cashBalance: string;
  assets: Record<AssetSymbol, AssetForm>;
}

interface PortfolioPageProps {
  portfolio: Portfolio | null;
  isLoading: boolean;
  onSaved: (portfolio: Portfolio) => void;
}

function assetForm(portfolio: Portfolio | null, symbol: AssetSymbol): AssetForm {
  const asset = portfolio?.assets.find((candidate) => candidate.symbol === symbol);
  return {
    quantity: String(asset?.quantity ?? 0),
    averageEntryPrice: String(asset?.averageEntryPrice ?? 0),
    cashAllocated: String(asset?.cashAllocated ?? 0),
  };
}

function formFromPortfolio(portfolio: Portfolio | null): PortfolioForm {
  return {
    cashBalance: String(portfolio?.cashBalance ?? 0),
    assets: { BTC: assetForm(portfolio, "BTC"), ETH: assetForm(portfolio, "ETH") },
  };
}

function nonNegativeNumber(value: string, label: string): number {
  const number = Number(value);
  if (value.trim() === "" || !Number.isFinite(number) || number < 0) {
    throw new Error(`${label} must be a non-negative number.`);
  }
  return number;
}

function savedAsset(symbol: AssetSymbol, values: AssetForm): PortfolioInput["assets"][number] {
  return {
    symbol,
    quantity: nonNegativeNumber(values.quantity, `${symbol} quantity`),
    averageEntryPrice: nonNegativeNumber(values.averageEntryPrice, `${symbol} average entry price`),
    cashAllocated: nonNegativeNumber(values.cashAllocated, `${symbol} cash allocated`),
  };
}

export function PortfolioPage({ portfolio, isLoading, onSaved }: PortfolioPageProps) {
  const [form, setForm] = useState<PortfolioForm>(() => formFromPortfolio(portfolio));
  const [error, setError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (portfolio) setForm(formFromPortfolio(portfolio));
  }, [portfolio]);

  const updateCash = (cashBalance: string) => setForm((current) => ({ ...current, cashBalance }));
  const updateAsset = (symbol: AssetSymbol, field: keyof AssetForm, value: string) => {
    setForm((current) => ({ ...current, assets: { ...current.assets, [symbol]: { ...current.assets[symbol], [field]: value } } }));
  };

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError(null);
    setSaved(false);

    let input: PortfolioInput;
    try {
      input = {
        cashBalance: nonNegativeNumber(form.cashBalance, "Cash balance"),
        assets: (["BTC", "ETH"] as AssetSymbol[]).map((symbol) => savedAsset(symbol, form.assets[symbol])),
      };
    } catch (validationError) {
      setError(validationError instanceof Error ? validationError.message : "Check the portfolio values.");
      return;
    }

    setIsSaving(true);
    try {
      await api.savePortfolio(input);
      const refreshed = await api.portfolio();
      onSaved(refreshed);
      setSaved(true);
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Could not save the portfolio.");
    } finally {
      setIsSaving(false);
    }
  }

  if (isLoading && !portfolio) {
    return <section className="placeholder"><h1>Portfolio</h1><p>Loading your locally stored portfolio.</p></section>;
  }

  return (
    <section className="portfolio-page">
      <header className="page-header">
        <div>
          <p className="eyebrow">Manual portfolio</p>
          <h1>Portfolio</h1>
        </div>
        <p className="page-note">This app records your holdings for advice only. It cannot place trades.</p>
      </header>

      <form className="portfolio-form" onSubmit={save}>
        <label className="field field--cash">
          <span>Cash balance (USD)</span>
          <input aria-label="Cash balance in USD" inputMode="decimal" min="0" onChange={(event) => updateCash(event.target.value)} step="any" type="number" value={form.cashBalance} />
        </label>

        <div className="portfolio-assets">
          {(["BTC", "ETH"] as AssetSymbol[]).map((symbol) => (
            <fieldset className="portfolio-asset" key={symbol}>
              <legend>{symbol}</legend>
              <label className="field">
                <span>Quantity</span>
                <input aria-label={`${symbol} quantity`} inputMode="decimal" min="0" onChange={(event) => updateAsset(symbol, "quantity", event.target.value)} step="any" type="number" value={form.assets[symbol].quantity} />
              </label>
              <label className="field">
                <span>Average entry price (USD)</span>
                <input aria-label={`${symbol} average entry price in USD`} inputMode="decimal" min="0" onChange={(event) => updateAsset(symbol, "averageEntryPrice", event.target.value)} step="any" type="number" value={form.assets[symbol].averageEntryPrice} />
              </label>
              <label className="field">
                <span>Cash allocated (USD)</span>
                <input aria-label={`${symbol} cash allocated in USD`} inputMode="decimal" min="0" onChange={(event) => updateAsset(symbol, "cashAllocated", event.target.value)} step="any" type="number" value={form.assets[symbol].cashAllocated} />
              </label>
            </fieldset>
          ))}
        </div>

        {error && <p className="form-message form-message--error" role="alert">{error}</p>}
        {saved && <p className="form-message form-message--success" role="status">Portfolio saved locally.</p>}
        <div className="form-actions"><button className="primary-button" disabled={isSaving} type="submit">{isSaving ? "Saving…" : "Save portfolio"}</button></div>
      </form>
    </section>
  );
}
