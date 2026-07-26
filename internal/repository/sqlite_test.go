package repository_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/yourusername/restaurant-finance/internal/config"
	"github.com/yourusername/restaurant-finance/internal/core"
	"github.com/yourusername/restaurant-finance/internal/repository"
	"github.com/yourusername/restaurant-finance/internal/service"
)

func TestSQLitePersistsFinanceData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restaurant-finance.db")
	cfg := &config.Config{
		DBDriver:   "sqlite",
		SQLitePath: path,
		DBMaxOpen:  4,
		DBMaxIdle:  2,
	}
	store, err := repository.ConnectDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	finance := service.NewFinanceService(store)
	employee := core.Employee{
		Name:        "Сотрудник",
		Position:    "Официант",
		KPIPercent:  decimal.RequireFromString("0.015"),
		HourlyRate:  decimal.NewFromInt(400),
		MonthlyRate: decimal.Zero,
		Active:      true,
	}
	if err := finance.ValidateEmployee(&employee); err == nil {
		t.Fatal("employee without restaurant must be rejected")
	}
	employee.RestaurantID = 999999
	if err := finance.ValidateEmployee(&employee); err == nil {
		t.Fatal("employee with unknown restaurant must be rejected")
	}

	restaurant := core.Restaurant{
		Name:     "SQLite Test",
		Currency: "RUB",
		Timezone: "Europe/Moscow",
	}
	if err := finance.CreateRestaurant(context.Background(), &restaurant); err != nil {
		t.Fatal(err)
	}
	employee.RestaurantID = restaurant.ID
	if err := finance.ValidateEmployee(&employee); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveEmployee(&employee); err != nil {
		t.Fatal(err)
	}
	employees, err := store.Employees(restaurant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(employees) != 1 || !employees[0].KPIPercent.Equal(decimal.RequireFromString("0.015")) {
		t.Fatalf("unexpected persisted employee: %#v", employees)
	}
	shiftDate := time.Date(2026, 7, 10, 0, 0, 0, 0, time.UTC)
	if err := store.SaveShift(&core.Shift{
		EmployeeID: employee.ID,
		Date:       shiftDate,
		Hours:      decimal.NewFromInt(8),
	}); err != nil {
		t.Fatal(err)
	}
	if err := store.DeactivateEmployee(restaurant.ID, employee.ID); err != nil {
		t.Fatal(err)
	}
	employees, err = store.Employees(restaurant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(employees) != 0 {
		t.Fatalf("deactivated employee is still active: %#v", employees)
	}
	belongs, err := store.EmployeeBelongsToRestaurant(restaurant.ID, employee.ID)
	if err != nil {
		t.Fatal(err)
	}
	if belongs {
		t.Fatal("deactivated employee must not accept new shifts")
	}
	payrollEmployees, err := store.PayrollEmployees(
		restaurant.ID,
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(payrollEmployees) != 1 || payrollEmployees[0].ID != employee.ID {
		t.Fatalf("deactivated employee disappeared from payroll history: %#v", payrollEmployees)
	}
	categories, err := store.Categories(restaurant.ID)
	if err != nil {
		t.Fatal(err)
	}
	var revenueID uint
	for _, category := range categories {
		if category.Kind == core.CategoryRevenue {
			revenueID = category.ID
			break
		}
	}
	if revenueID == 0 {
		t.Fatal("default revenue category was not created")
	}
	entry := core.FinancialEntry{
		RestaurantID: restaurant.ID,
		CategoryID:   &revenueID,
		OccurredAt:   time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC),
		Amount:       decimal.NewFromInt(125000),
		Direction:    "income",
	}
	if err := finance.SaveManualEntry(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}
	entry.Description = "Обновлённая операция"
	entry.Tags = "закупка, срочно"
	if err := finance.UpdateManualEntry(context.Background(), &entry); err != nil {
		t.Fatal(err)
	}
	updatedEntries, err := store.Entries(context.Background(), repository.EntryFilter{
		RestaurantID: restaurant.ID,
		From:         time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		To:           time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	})
	if err != nil || len(updatedEntries) != 1 || updatedEntries[0].Tags != "закупка, срочно" {
		t.Fatalf("operation update failed: entries=%#v err=%v", updatedEntries, err)
	}
	closeStore(t, store)

	reopened, err := repository.ConnectDB(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer closeStore(t, reopened)
	values, err := reopened.Restaurants()
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0].ID != restaurant.ID {
		t.Fatalf("unexpected persisted restaurants: %#v", values)
	}
	report, err := service.NewFinanceService(reopened).PnL(
		context.Background(),
		restaurant.ID,
		time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Revenue.Equal(decimal.NewFromInt(125000)) {
		t.Fatalf("revenue = %s, want 125000", report.Revenue)
	}
}

func closeStore(t *testing.T, store *repository.Store) {
	t.Helper()
	sqlDB, err := store.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
}
