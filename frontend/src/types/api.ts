export type AssetSymbol = "BTC" | "ETH";

export interface AssetPosition {
  symbol: AssetSymbol;
  quantity: number;
  averageEntryPrice: number;
  cashAllocated: number;
  updatedAt: string;
}

export interface Portfolio {
  cashBalance: number;
  assets: AssetPosition[];
}

export interface MarketSnapshot {
  symbol: AssetSymbol;
  price: number;
  updatedAt: string;
}

export interface StrategyResult {
  asset: AssetSymbol;
  action: string;
  actionPct: number;
  state: string;
  price: number;
  trend: string;
  positionPct: number;
  profitLossPct: number;
  pullbackPct: number;
  localHigh: number;
  localLow: number;
  drawdownFromHighPct: number;
  reason: string;
  nextCondition: string;
}

export interface StrategyConfig {
  pullbackMinPct: number;
  pivotLeft: number;
  pivotRight: number;
  breakoutBufferPct: number;
  trendMode: "STRICT" | "RECOVERY" | "OFF";
  entry1Pct: number;
  entry2Pct: number;
  entry3Pct: number;
  profitTrigger1Pct: number;
  profitTrigger2Pct: number;
  profitTakePct: number;
  drawdown1Pct: number;
  drawdown1SellPct: number;
  drawdown2Pct: number;
  drawdown2SellPct: number;
  drawdown3Pct: number;
  drawdown3SellPct: number;
}
