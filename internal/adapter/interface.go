package adapter

import (
	"context"
	"time"

	"github.com/yourusername/restaurant-finance/internal/core"
)

type POSAdapter interface {
	Provider() string
	Test(ctx context.Context, connection core.POSConnection) error
	Fetch(ctx context.Context, connection core.POSConnection, from, to time.Time) ([]core.FinancialEntry, error)
}

type Registry struct {
	adapters map[string]POSAdapter
}

func NewRegistry(values ...POSAdapter) *Registry {
	result := &Registry{adapters: map[string]POSAdapter{}}
	for _, value := range values {
		result.adapters[value.Provider()] = value
	}
	return result
}

func (r *Registry) Get(provider string) (POSAdapter, bool) {
	value, ok := r.adapters[provider]
	return value, ok
}
