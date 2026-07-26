package service

import (
	"context"
	"io"
	"time"

	"github.com/yourusername/restaurant-finance/internal/adapter/excel"
	"github.com/yourusername/restaurant-finance/internal/core"
)

type ExcelService struct {
	finance *FinanceService
}

func NewExcelService(finance *FinanceService) *ExcelService {
	return &ExcelService{finance: finance}
}

func (s *ExcelService) Preview(reader io.Reader) (excel.Preview, error) {
	return excel.ReadPreview(reader, 100)
}

func (s *ExcelService) Import(ctx context.Context, reader io.Reader, restaurantID uint, mapping excel.ColumnMapping) (excel.ImportResult, error) {
	categories, err := s.finance.store.Categories(restaurantID)
	if err != nil {
		return excel.ImportResult{}, err
	}
	result, err := excel.Import(reader, restaurantID, mapping, categories)
	if err != nil {
		return result, err
	}
	rules, err := s.finance.store.Rules(restaurantID)
	if err != nil {
		return result, err
	}
	engine := core.NewRuleEngine(rules)
	for index := range result.Entries {
		engine.Classify(&result.Entries[index])
	}
	result.Imported, err = s.finance.store.SaveEntries(ctx, result.Entries, 500)
	result.Skipped = len(result.Entries) - result.Imported
	return result, err
}

func (s *ExcelService) ImportTemplate(ctx context.Context, reader io.Reader, restaurantID uint, month time.Time) (excel.ImportResult, error) {
	categories, err := s.finance.store.Categories(restaurantID)
	if err != nil {
		return excel.ImportResult{}, err
	}
	result, err := excel.ImportRestaurantTemplate(reader, restaurantID, month, categories)
	if err != nil {
		return result, err
	}
	rules, err := s.finance.store.Rules(restaurantID)
	if err != nil {
		return result, err
	}
	engine := core.NewRuleEngine(rules)
	for index := range result.Entries {
		engine.Classify(&result.Entries[index])
	}
	result.Imported, err = s.finance.store.SaveEntries(ctx, result.Entries, 500)
	result.Skipped = len(result.Entries) - result.Imported
	return result, err
}

func (s *ExcelService) Export(ctx context.Context, writer io.Writer, restaurantID uint, from, to time.Time) error {
	dashboard, err := s.finance.Dashboard(ctx, restaurantID, from, to)
	if err != nil {
		return err
	}
	return excel.ExportReports(writer, dashboard.PnL, dashboard.CashFlow, dashboard.Payroll)
}

func (s *ExcelService) ExportPayroll(ctx context.Context, writer io.Writer, restaurantID uint, from, to time.Time) error {
	payroll, err := s.finance.Payroll(ctx, restaurantID, from, to)
	if err != nil {
		return err
	}
	return excel.ExportPayroll(writer, payroll)
}
