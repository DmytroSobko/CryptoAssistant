export type AssetSymbol = "BTC" | "ETH";
export type StrategyID = "RECOVERY_BREAKOUT" | "DIP_ACCUMULATION";

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

export interface PortfolioInput {
  cashBalance: number;
  assets: Array<Omit<AssetPosition, "updatedAt">>;
}

export interface MarketSnapshot {
  symbol: AssetSymbol;
  price: number;
  updatedAt: string;
}

export interface StrategyResult {
  asset: AssetSymbol;
  strategyId?: StrategyID;
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
  breakEvenFloorNetPrice?: number;
  breakEvenFloorGrossPrice?: number;
  intradayAlert?: boolean;
  recommendationId?: number;
  recommendationStatus?: string;
  historicalPeakAverage?: number;
  reason: string;
  nextCondition: string;
}

export interface StrategyEvent {
  id: number;
  asset: AssetSymbol;
  strategyId?: StrategyID;
  timestamp: string;
  action: string;
  price: number;
  reason: string;
  state: string;
}

export interface StrategyConfig {
  strategyId?: StrategyID;
  pullbackMinPct: number;
  recoveryEntryMinPeakDiscountPct?: number;
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
  entry2DipFromFirstPct?: number;
  entry3DipFromFirstPct?: number;
  estimatedSellFeeBps?: number;
  breakEvenExitFloorEnabled?: boolean;
  intradayDrawdownAlertsEnabled?: boolean;
  // Copied from the asset-level setting for a live evaluation or backtest run.
  athEntryOverrideEnabled?: boolean;
  athEntryThresholdPct?: number;
  athReferencePeak?: number;
  athPeakCount?: number;
  athSourceFile?: string;
}

export interface ATHEntryOverrideSettings {
  enabled: boolean;
  thresholdPct: number;
  sourceFile?: string;
  peakCount?: number;
}

export interface AssetStrategySettings {
  selectedStrategyId: StrategyID;
  recoveryBreakout: StrategyConfig;
  dipAccumulation: StrategyConfig;
  athEntryOverride: ATHEntryOverrideSettings;
}

export type BacktestExecutionModel = "NEXT_DAILY_OPEN";

export interface BacktestCandleSet {
  id: string;
  asset: AssetSymbol;
  sourceLabel: string;
  sourceFilename: string;
  originalSha256: string;
  schemaVersion: number;
  candleCount: number;
  firstTimestamp: string;
  lastTimestamp: string;
  importedAt: string;
}

export interface BacktestImportInput {
  asset: AssetSymbol;
  sourceLabel: string;
  sourceFilename: string;
  csv: string;
}

export interface HistoricalPeakSourceUpload {
  sourceFile: string;
}

export interface BacktestRunInput {
  candleSetId: string;
  asset: AssetSymbol;
  start: string;
  end: string;
  startingCashUsd: number;
  strategyConfig: StrategyConfig;
  feeBps: number;
  slippageBps: number;
  executionModel: BacktestExecutionModel;
}

export interface BacktestAssumptions {
  recoveryEntryPeakGate?: string;
  executionModel: BacktestExecutionModel;
  signalTiming: string;
  fillTiming: string;
  buySizing: string;
  sellSizing: string;
  feeBps: number;
  slippageBps: number;
  noMarginOrBorrowing: boolean;
  taxesIncluded: boolean;
  profitTrigger2Implementation: string;
  referencePricePolicy: string;
  breakEvenExitFloor: string;
  athEntryOverride?: string;
}

export interface BacktestSummary {
  startingCashUsd: number;
  endingCashUsd: number;
  endingAssetQuantity: number;
  endingAssetValueUsd: number;
  endingEquityUsd: number;
  netProfitLossUsd: number;
  returnPct: number;
  maximumDrawdownPct: number;
  firstDataTimestamp: string;
  lastDataTimestamp: string;
  warmupStart: string;
  signalCount: number;
  executedTradeCount: number;
  rejectedTradeCount: number;
  unexecutedTradeCount: number;
  totalFeesUsd: number;
  buyAndHoldEndingUsd: number;
  buyAndHoldReturnPct: number;
  breakEvenFloorBlockedTradeCount: number;
}

export interface BacktestSignal {
  sequence: number;
  timestamp: string;
  decisionClose: number;
  action: string;
  actionPct: number;
  strategyState: string;
  reason: string;
  nextCondition: string;
  orderStatus: string;
  breakEvenFloorNetPrice?: number;
}

export interface BacktestTrade {
  sequence: number;
  signalSequence: number;
  signalTimestamp: string;
  executionTimestamp: string;
  side: "BUY" | "SELL";
  action: string;
  requestedPct: number;
  status: string;
  rawOpenPrice: number;
  fillPrice: number;
  quantity: number;
  grossNotionalUsd: number;
  feeUsd: number;
  cashAfterUsd: number;
  quantityAfter: number;
  averageEntryAfter: number;
  breakEvenFloorNetPrice?: number;
  reason: string;
}

export interface BacktestEquityPoint {
  timestamp: string;
  closePrice: number;
  cashUsd: number;
  quantity: number;
  assetValueUsd: number;
  equityUsd: number;
  peakEquityUsd: number;
  drawdownPct: number;
}

export interface BacktestResult {
  request: Omit<BacktestRunInput, "candleSetId">;
  dataFingerprint: string;
  assumptions: BacktestAssumptions;
  summary: BacktestSummary;
  signals: BacktestSignal[];
  trades: BacktestTrade[];
  equityCurve: BacktestEquityPoint[];
}

export interface BacktestRun {
  id: string;
  candleSetId: string;
  createdAt: string;
  completedAt: string;
  result: BacktestResult;
}

export interface BacktestRunSummary {
  id: string;
  candleSetId: string;
  asset: AssetSymbol;
  start: string;
  end: string;
  startingCashUsd: number;
  returnPct: number;
  maximumDrawdownPct: number;
  strategyId?: StrategyID;
  createdAt: string;
  completedAt: string;
}
