import type { AssetStrategySettings, AssetSymbol, BacktestCandleSet, BacktestImportInput, BacktestRun, BacktestRunInput, BacktestRunSummary, MarketSnapshot, Portfolio, PortfolioInput, StrategyConfig, StrategyEvent, StrategyResult } from "../types/api";

const API_BASE_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

export class APIError extends Error {
  constructor(message: string, public readonly status: number) {
    super(message);
  }
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const headers = new Headers(options?.headers);
  headers.set("Content-Type", "application/json");
  const response = await fetch(`${API_BASE_URL}${path}`, {
    ...options,
    headers,
  });
  if (!response.ok) {
    const body = (await response.json().catch(() => null)) as { error?: string } | null;
    throw new APIError(body?.error ?? `Request failed (${response.status})`, response.status);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export const api = {
  health: () => request<{ status: string }>("/healthz"),
  market: (asset: AssetSymbol) => request<MarketSnapshot>(`/api/market/${asset}`),
  refreshMarket: () => request<void>("/api/market/refresh", { method: "POST" }),
  strategy: (asset: AssetSymbol) => request<StrategyResult>(`/api/strategy/${asset}`),
  history: () => request<StrategyEvent[]>("/api/history"),
  portfolio: () => request<Portfolio>("/api/portfolio"),
  savePortfolio: (portfolio: PortfolioInput) => request<void>("/api/portfolio", { method: "PUT", body: JSON.stringify(portfolio) }),
  config: () => request<Record<"BTC" | "ETH", StrategyConfig>>("/api/config"),
  saveConfig: (asset: "BTC" | "ETH", config: StrategyConfig) => request<void>(`/api/config/${asset}`, { method: "PUT", body: JSON.stringify(config) }),
  strategySettings: () => request<Record<AssetSymbol, AssetStrategySettings>>("/api/strategy-settings"),
  saveStrategySettings: (asset: AssetSymbol, settings: AssetStrategySettings) => request<void>(`/api/strategy-settings/${asset}`, { method: "PUT", body: JSON.stringify(settings) }),
  importBacktestCandleSet: (input: BacktestImportInput) => request<BacktestCandleSet>("/api/backtest/candle-sets", { method: "POST", body: JSON.stringify(input) }),
  backtestCandleSets: (asset?: AssetSymbol) => request<BacktestCandleSet[]>(`/api/backtest/candle-sets${asset ? `?asset=${asset}` : ""}`),
  createBacktest: (input: BacktestRunInput) => request<BacktestRun>("/api/backtests", { method: "POST", body: JSON.stringify(input) }),
  backtests: () => request<BacktestRunSummary[]>("/api/backtests"),
  backtest: (id: string) => request<BacktestRun>(`/api/backtests/${encodeURIComponent(id)}`),
  deleteBacktest: (id: string) => request<void>(`/api/backtests/${encodeURIComponent(id)}`, { method: "DELETE" }),
};
