package repository

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/yourusername/restaurant-finance/internal/core"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Store) CreateRestaurant(value *core.Restaurant) error {
	return s.DB.Create(value).Error
}

func (s *Store) Restaurants() ([]core.Restaurant, error) {
	var values []core.Restaurant
	return values, s.DB.Order("name").Find(&values).Error
}

func (s *Store) RestaurantExists(restaurantID uint) (bool, error) {
	var count int64
	err := s.DB.Model(&core.Restaurant{}).Where("id = ?", restaurantID).Count(&count).Error
	return count == 1, err
}

func (s *Store) CreateCategory(value *core.FinancialCategory) error {
	return s.DB.Create(value).Error
}

func (s *Store) CreateCategories(values []core.FinancialCategory) error {
	if len(values) == 0 {
		return nil
	}
	return s.DB.CreateInBatches(&values, 100).Error
}

func (s *Store) Categories(restaurantID uint) ([]core.FinancialCategory, error) {
	var values []core.FinancialCategory
	return values, s.DB.Where("restaurant_id = ?", restaurantID).Order("sort_order, name").Find(&values).Error
}

func (s *Store) CategoryBelongsToRestaurant(restaurantID, categoryID uint) (bool, error) {
	var count int64
	err := s.DB.Model(&core.FinancialCategory{}).
		Where("id = ? AND restaurant_id = ?", categoryID, restaurantID).
		Count(&count).Error
	return count == 1, err
}

func (s *Store) SaveEntry(ctx context.Context, value *core.FinancialEntry) (bool, error) {
	db := s.DB.WithContext(ctx)
	if value.Source == "" {
		value.Source = "manual"
	}
	if value.ExternalID != "" {
		result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(value)
		return result.RowsAffected == 1, result.Error
	}
	return true, db.Create(value).Error
}

func (s *Store) UpdateEntry(ctx context.Context, value *core.FinancialEntry) error {
	result := s.DB.WithContext(ctx).
		Where("id = ? AND restaurant_id = ?", value.ID, value.RestaurantID).
		Select("category_id", "occurred_at", "amount", "direction", "payment_method", "description", "counterparty", "tags", "updated_at").
		Updates(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) SaveEntries(ctx context.Context, values []core.FinancialEntry, batchSize int) (int, error) {
	if len(values) == 0 {
		return 0, nil
	}
	if batchSize <= 0 {
		batchSize = 500
	}
	result := s.DB.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(&values, batchSize)
	return int(result.RowsAffected), result.Error
}

type EntryFilter struct {
	RestaurantID uint
	From         time.Time
	To           time.Time
	CategoryID   *uint
	Direction    string
	Query        string
	Limit        int
	Offset       int
}

func (s *Store) Entries(ctx context.Context, filter EntryFilter) ([]core.FinancialEntry, error) {
	if filter.Limit <= 0 || filter.Limit > 500 {
		filter.Limit = 200
	}
	var values []core.FinancialEntry
	query := s.DB.WithContext(ctx).
		Where("restaurant_id = ? AND occurred_at >= ? AND occurred_at < ?", filter.RestaurantID, filter.From, filter.To.AddDate(0, 0, 1))
	if filter.CategoryID != nil {
		query = query.Where("category_id = ?", *filter.CategoryID)
	}
	if filter.Direction != "" {
		query = query.Where("direction = ?", filter.Direction)
	}
	if filter.Query != "" {
		search := "%" + filter.Query + "%"
		query = query.Where(
			"LOWER(description) LIKE LOWER(?) OR LOWER(counterparty) LIKE LOWER(?)",
			search,
			search,
		)
	}
	err := query.Order("occurred_at DESC, id DESC").
		Limit(filter.Limit).
		Offset(filter.Offset).
		Find(&values).Error
	return values, err
}

func (s *Store) EntryTotals(ctx context.Context, restaurantID uint, from, to time.Time) (map[uint]decimal.Decimal, error) {
	var rows []struct {
		CategoryID uint
		Amount     decimal.Decimal
	}
	err := s.DB.WithContext(ctx).
		Model(&core.FinancialEntry{}).
		Select(`
			financial_entries.category_id,
			SUM(
				CASE
					WHEN (
						financial_categories.kind IN ('revenue', 'cash_in')
						AND financial_entries.direction = 'income'
					) OR (
						financial_categories.kind NOT IN ('revenue', 'cash_in')
						AND financial_entries.direction = 'expense'
					)
					THEN financial_entries.amount
					ELSE -financial_entries.amount
				END
			) AS amount
		`).
		Joins("JOIN financial_categories ON financial_categories.id = financial_entries.category_id").
		Where(
			"financial_entries.restaurant_id = ? AND financial_entries.occurred_at >= ? AND financial_entries.occurred_at < ?",
			restaurantID,
			from,
			to.AddDate(0, 0, 1),
		).
		Group("financial_entries.category_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[uint]decimal.Decimal, len(rows))
	for _, row := range rows {
		result[row.CategoryID] = row.Amount
	}
	return result, nil
}

func (s *Store) DeleteEntry(restaurantID, id uint) error {
	result := s.DB.Where("restaurant_id = ? AND id = ?", restaurantID, id).Delete(&core.FinancialEntry{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) UpsertPlan(value *core.PlanValue) error {
	return s.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "restaurant_id"}, {Name: "category_id"}, {Name: "month"}},
		DoUpdates: clause.AssignmentColumns([]string{"amount", "updated_at"}),
	}).Create(value).Error
}

func (s *Store) Plans(restaurantID uint, from, to time.Time) ([]core.PlanValue, error) {
	var values []core.PlanValue
	err := s.DB.Where("restaurant_id = ? AND month >= ? AND month <= ?", restaurantID, monthStart(from), monthStart(to)).
		Find(&values).Error
	return values, err
}

func (s *Store) CreateRule(value *core.CalculationRule) error {
	return s.DB.Create(value).Error
}

func (s *Store) UpdateRule(value *core.CalculationRule) error {
	result := s.DB.Where("restaurant_id = ? AND id = ?", value.RestaurantID, value.ID).Updates(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Rules(restaurantID uint) ([]core.CalculationRule, error) {
	var values []core.CalculationRule
	return values, s.DB.Where("restaurant_id = ?", restaurantID).Order("priority, id").Find(&values).Error
}

func (s *Store) SaveEmployee(value *core.Employee) error {
	if value.ID == 0 {
		return s.DB.Create(value).Error
	}
	result := s.DB.
		Where("id = ? AND restaurant_id = ?", value.ID, value.RestaurantID).
		Select("name", "position", "hourly_rate", "monthly_rate", "kpi_percent", "active", "updated_at").
		Updates(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Employees(restaurantID uint) ([]core.Employee, error) {
	var values []core.Employee
	return values, s.DB.Where("restaurant_id = ? AND active = ?", restaurantID, true).Order("position, name").Find(&values).Error
}

func (s *Store) InactiveEmployees(restaurantID uint) ([]core.Employee, error) {
	var values []core.Employee
	return values, s.DB.Where("restaurant_id = ? AND active = ?", restaurantID, false).Order("position, name").Find(&values).Error
}

func (s *Store) AllEmployees(restaurantID uint) ([]core.Employee, error) {
	var values []core.Employee
	return values, s.DB.Where("restaurant_id = ?", restaurantID).Order("active DESC, position, name").Find(&values).Error
}

func (s *Store) PayrollEmployees(restaurantID uint, from, to time.Time) ([]core.Employee, error) {
	var values []core.Employee
	err := s.DB.Model(&core.Employee{}).
		Distinct("employees.*").
		Joins("LEFT JOIN shifts ON shifts.employee_id = employees.id AND shifts.date >= ? AND shifts.date <= ?", from, to).
		Where("employees.restaurant_id = ? AND (employees.active = ? OR shifts.id IS NOT NULL)", restaurantID, true).
		Order("employees.position, employees.name").
		Find(&values).Error
	return values, err
}

func (s *Store) DeactivateEmployee(restaurantID, employeeID uint) error {
	result := s.DB.Model(&core.Employee{}).
		Where("id = ? AND restaurant_id = ? AND active = ?", employeeID, restaurantID, true).
		Update("active", false)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) EmployeeBelongsToRestaurant(restaurantID, employeeID uint) (bool, error) {
	var count int64
	err := s.DB.Model(&core.Employee{}).
		Where("id = ? AND restaurant_id = ? AND active = ?", employeeID, restaurantID, true).
		Count(&count).Error
	return count == 1, err
}

func (s *Store) SaveShift(value *core.Shift) error {
	if value.ID == 0 {
		return s.DB.Create(value).Error
	}
	result := s.DB.
		Where("id = ? AND employee_id = ?", value.ID, value.EmployeeID).
		Select("date", "hours", "bonus", "advance", "deduction", "comment", "updated_at").
		Updates(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) Shifts(restaurantID uint, from, to time.Time) ([]core.Shift, error) {
	var values []core.Shift
	err := s.DB.Joins("JOIN employees ON employees.id = shifts.employee_id").
		Where("employees.restaurant_id = ? AND shifts.date >= ? AND shifts.date <= ?", restaurantID, from, to).
		Order("shifts.date").
		Find(&values).Error
	return values, err
}

func (s *Store) SavePOSConnection(value *core.POSConnection) error {
	if value.ID == 0 {
		return s.DB.Create(value).Error
	}
	result := s.DB.
		Where("id = ? AND restaurant_id = ?", value.ID, value.RestaurantID).
		Select("provider", "name", "base_url", "api_key", "settings", "active", "last_sync_at", "updated_at").
		Updates(value)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) POSConnections(restaurantID uint) ([]core.POSConnection, error) {
	var values []core.POSConnection
	return values, s.DB.Where("restaurant_id = ?", restaurantID).Order("name").Find(&values).Error
}

func (s *Store) POSConnection(restaurantID, id uint) (core.POSConnection, error) {
	var value core.POSConnection
	err := s.DB.Where("restaurant_id = ? AND id = ?", restaurantID, id).First(&value).Error
	return value, err
}

func (s *Store) Transaction(ctx context.Context, fn func(*Store) error) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error { return fn(&Store{DB: tx}) })
}

func monthStart(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), 1, 0, 0, 0, 0, value.Location())
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
