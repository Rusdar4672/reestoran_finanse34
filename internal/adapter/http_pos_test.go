package adapter

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourusername/restaurant-finance/internal/core"
)

func TestHTTPPOSAdapterFetch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") != "2026-07-01" || r.URL.Query().Get("to") != "2026-07-31" {
			t.Errorf("unexpected period: %s", r.URL.RawQuery)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization header is missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{
			"id":"sale-1",
			"date":"2026-07-01T12:00:00Z",
			"amount":"1250.50",
			"direction":"income",
			"payment_method":"card",
			"description":"Кухня"
		}]`))
	}))
	defer server.Close()

	adapter := NewHTTPPOSAdapter("generic")
	rows, err := adapter.Fetch(
		context.Background(),
		core.POSConnection{RestaurantID: 7, BaseURL: server.URL, APIKey: "secret"},
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ExternalID != "sale-1" || rows[0].RestaurantID != 7 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestHTTPPOSAdapterRejectsInvalidRow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"","date":"2026-07-01T12:00:00Z","amount":"10","direction":"income"}]`))
	}))
	defer server.Close()

	_, err := NewHTTPPOSAdapter("generic").Fetch(
		context.Background(),
		core.POSConnection{BaseURL: server.URL},
		time.Now(),
		time.Now(),
	)
	if err == nil {
		t.Fatal("expected invalid POS row error")
	}
}
