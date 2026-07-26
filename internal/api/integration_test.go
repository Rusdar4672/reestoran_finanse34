package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/yourusername/restaurant-finance/internal/bootstrap"
	"github.com/yourusername/restaurant-finance/internal/config"
	"github.com/yourusername/restaurant-finance/internal/core"
)

func TestRestaurantDashboardFlow(t *testing.T) {
	if os.Getenv("INTEGRATION_DB") != "1" {
		t.Skip("set INTEGRATION_DB=1 to run PostgreSQL integration tests")
	}
	if err := godotenv.Overload("../../.env"); err != nil {
		t.Fatal(err)
	}
	app, err := bootstrap.New(config.LoadConfig(), "*")
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	name := fmt.Sprintf("integration-%d", time.Now().UnixNano())
	var restaurant core.Restaurant
	doJSON(t, app.Router, http.MethodPost, "/api/v1/restaurants", map[string]any{
		"name": name, "currency": "RUB",
	}, http.StatusCreated, &restaurant)
	t.Cleanup(func() {
		_ = app.Store.DB.Unscoped().Delete(&core.Restaurant{}, restaurant.ID).Error
	})

	var categories []core.FinancialCategory
	doJSON(t, app.Router, http.MethodGet, fmt.Sprintf("/api/v1/restaurants/%d/categories", restaurant.ID), nil, http.StatusOK, &categories)
	var revenueID uint
	for _, category := range categories {
		if category.Code == "revenue_kitchen" {
			revenueID = category.ID
			break
		}
	}
	if revenueID == 0 {
		t.Fatal("default revenue category was not created")
	}

	doJSON(t, app.Router, http.MethodPost, fmt.Sprintf("/api/v1/restaurants/%d/entries", restaurant.ID), map[string]any{
		"category_id": revenueID,
		"date":        "2026-07-01",
		"amount":      "1000",
		"direction":   "income",
	}, http.StatusCreated, nil)

	var dashboard struct {
		PnL struct {
			Revenue string `json:"revenue"`
		} `json:"pnl"`
	}
	doJSON(
		t,
		app.Router,
		http.MethodGet,
		fmt.Sprintf("/api/v1/restaurants/%d/reports/dashboard?from=2026-07-01&to=2026-07-31", restaurant.ID),
		nil,
		http.StatusOK,
		&dashboard,
	)
	if dashboard.PnL.Revenue != "1000" {
		t.Fatalf("revenue = %s", dashboard.PnL.Revenue)
	}
}

func doJSON(t *testing.T, handler http.Handler, method, path string, body any, wantStatus int, target any) {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	request := httptest.NewRequest(method, path, &payload)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != wantStatus {
		t.Fatalf("%s %s: status %d, body %s", method, path, response.Code, response.Body.String())
	}
	if target != nil {
		if err := json.NewDecoder(response.Body).Decode(target); err != nil {
			t.Fatal(err)
		}
	}
}
