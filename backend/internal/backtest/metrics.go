package backtest

import "github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/market"

func equityPoint(candle market.Candle, state SimulationState, peak float64) EquityPoint {
	assetValue := state.Position.Quantity * candle.Close
	equity := state.CashUSD + assetValue
	if equity > peak {
		peak = equity
	}
	drawdown := 0.0
	if peak > 0 {
		drawdown = (equity - peak) / peak * 100
	}
	return EquityPoint{Timestamp: candle.Timestamp, ClosePrice: candle.Close, CashUSD: state.CashUSD, Quantity: state.Position.Quantity, AssetValueUSD: assetValue, EquityUSD: equity, PeakEquityUSD: peak, DrawdownPct: drawdown}
}

func buildSummary(result *Result, candles []market.Candle, startIndex, endIndex int) Summary {
	last := result.EquityCurve[len(result.EquityCurve)-1]
	summary := Summary{
		StartingCashUSD: result.Request.StartingCashUSD, EndingCashUSD: last.CashUSD,
		EndingAssetQuantity: last.Quantity, EndingAssetValueUSD: last.AssetValueUSD, EndingEquityUSD: last.EquityUSD,
		NetProfitLossUSD:   last.EquityUSD - result.Request.StartingCashUSD,
		ReturnPct:          (last.EquityUSD - result.Request.StartingCashUSD) / result.Request.StartingCashUSD * 100,
		FirstDataTimestamp: candles[startIndex].Timestamp, LastDataTimestamp: candles[endIndex].Timestamp,
		WarmupStart: candles[0].Timestamp, SignalCount: len(result.Signals),
	}
	for _, point := range result.EquityCurve {
		if point.DrawdownPct < summary.MaximumDrawdownPct {
			summary.MaximumDrawdownPct = point.DrawdownPct
		}
	}
	for _, trade := range result.Trades {
		summary.TotalFeesUSD += trade.FeeUSD
		if trade.Status == "EXECUTED" {
			summary.ExecutedTradeCount++
		} else {
			summary.RejectedTradeCount++
		}
	}
	for _, signal := range result.Signals {
		if signal.OrderStatus == "UNEXECUTED_END_OF_DATA" {
			summary.UnexecutedTradeCount++
		}
	}
	buyAndHoldQuantity := result.Request.StartingCashUSD / candles[startIndex].Close
	summary.BuyAndHoldEndingUSD = buyAndHoldQuantity * candles[endIndex].Close
	summary.BuyAndHoldReturnPct = (summary.BuyAndHoldEndingUSD - result.Request.StartingCashUSD) / result.Request.StartingCashUSD * 100
	return summary
}
