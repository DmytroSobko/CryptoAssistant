import type { Portfolio, StrategyConfig } from "../types/api";

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
  portfolio: () => request<Portfolio>("/api/portfolio"),
  savePortfolio: (portfolio: Portfolio) => request<void>("/api/portfolio", { method: "PUT", body: JSON.stringify(portfolio) }),
  config: () => request<Record<"BTC" | "ETH", StrategyConfig>>("/api/config"),
  saveConfig: (asset: "BTC" | "ETH", config: StrategyConfig) => request<void>(`/api/config/${asset}`, { method: "PUT", body: JSON.stringify(config) }),
};
