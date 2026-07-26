package service

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/restaurant-finance/internal/adapter"
	"github.com/yourusername/restaurant-finance/internal/core"
)

type POSService struct {
	finance  *FinanceService
	registry *adapter.Registry
}

func NewPOSService(finance *FinanceService, registry *adapter.Registry) *POSService {
	return &POSService{finance: finance, registry: registry}
}

func (s *POSService) Supports(provider string) bool {
	_, ok := s.registry.Get(provider)
	return ok
}

func (s *POSService) Test(ctx context.Context, restaurantID, connectionID uint) error {
	connection, err := s.finance.store.POSConnection(restaurantID, connectionID)
	if err != nil {
		return err
	}
	pos, ok := s.registry.Get(connection.Provider)
	if !ok {
		return fmt.Errorf("POS provider %q is not registered", connection.Provider)
	}
	return pos.Test(ctx, connection)
}

func (s *POSService) Sync(ctx context.Context, restaurantID, connectionID uint, from, to time.Time) (int, error) {
	connection, err := s.finance.store.POSConnection(restaurantID, connectionID)
	if err != nil {
		return 0, err
	}
	pos, ok := s.registry.Get(connection.Provider)
	if !ok {
		return 0, fmt.Errorf("POS provider %q is not registered", connection.Provider)
	}
	rows, err := pos.Fetch(ctx, connection, from, to)
	if err != nil {
		return 0, err
	}
	rules, err := s.finance.store.Rules(restaurantID)
	if err != nil {
		return 0, err
	}
	engine := core.NewRuleEngine(rules)
	for i := range rows {
		engine.Classify(&rows[i])
	}
	imported, err := s.finance.store.SaveEntries(ctx, rows, 500)
	if err != nil {
		return imported, err
	}
	now := time.Now()
	connection.LastSyncAt = &now
	if err := s.finance.store.SavePOSConnection(&connection); err != nil {
		return imported, err
	}
	return imported, nil
}
