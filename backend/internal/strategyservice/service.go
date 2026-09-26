// Package strategyservice coordinates persistence with the pure strategy
// engine. It deliberately contains no HTTP or market-provider concerns.
package strategyservice

import (
	"context"

	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/storage"
	"github.com/dmytrosobko/crypto-strategy-assistant/backend/internal/strategy"
)

type Service struct {
	store *storage.Store
}

func New(store *storage.Store) *Service {
	return &Service{store: store}
}

// Evaluate loads only persisted completed daily candles, the manually entered
// position, configuration, and state; then it atomically persists the result.
func (s *Service) Evaluate(ctx context.Context, asset string) (strategy.Result, error) {
	return s.store.EvaluateStrategyAtomically(ctx, asset, func(input storage.StrategyEvaluationInput) (strategy.Result, strategy.PersistedState) {
		return strategy.Evaluate(input.Candles, input.Position, input.Config, input.State)
	})
}
