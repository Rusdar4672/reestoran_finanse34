package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/yourusername/restaurant-finance/internal/core"
	"github.com/yourusername/restaurant-finance/internal/repository"
	"golang.org/x/sync/errgroup"
)

type FinanceService struct {
	store *repository.Store
}

type Dashboard struct {
	PnL       core.PnLReport      `json:"pnl"`
	CashFlow  core.CashFlowReport `json:"cash_flow"`
	Payroll   core.PayrollReport  `json:"payroll"`
	BreakEven decimal.Decimal     `json:"break_even_revenue"`
}

var categoryCodePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,99}$`)

func NewFinanceService(store *repository.Store) *FinanceService {
	return &FinanceService{store: store}
}

func (s *FinanceService) Store() *repository.Store {
	return s.store
}

func (s *FinanceService) CreateRestaurant(ctx context.Context, value *core.Restaurant) error {
	return s.store.Transaction(ctx, func(store *repository.Store) error {
		if err := store.CreateRestaurant(value); err != nil {
			return err
		}
		categories := defaultCategories()
		for index := range categories {
			categories[index].RestaurantID = value.ID
			categories[index].SortOrder = index * 10
			categories[index].Active = true
		}
		return store.CreateCategories(categories)
	})
}

func (s *FinanceService) SaveManualEntry(ctx context.Context, entry *core.FinancialEntry) error {
	if err := s.validateManualEntry(ctx, entry); err != nil {
		return err
	}
	_, err := s.store.SaveEntry(ctx, entry)
	return err
}

func (s *FinanceService) validateManualEntry(ctx context.Context, entry *core.FinancialEntry) error {
	if entry.RestaurantID == 0 {
		return errors.New("restaurant_id is required")
	}
	if entry.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}
	if entry.Amount.LessThanOrEqual(decimal.Zero) {
		return errors.New("amount must be greater than zero")
	}
	if entry.Direction != "income" && entry.Direction != "expense" {
		return errors.New("direction must be income or expense")
	}
	if entry.CategoryID != nil {
		ok, err := s.store.CategoryBelongsToRestaurant(entry.RestaurantID, *entry.CategoryID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("category does not belong to restaurant")
		}
	}
	entry.Source = "manual"
	rules, err := s.store.Rules(entry.RestaurantID)
	if err != nil {
		return err
	}
	if entry.CategoryID == nil {
		core.NewRuleEngine(rules).Classify(entry)
	}
	return nil
}

func (s *FinanceService) UpdateManualEntry(ctx context.Context, entry *core.FinancialEntry) error {
	if err := s.validateManualEntry(ctx, entry); err != nil {
		return err
	}
	return s.store.UpdateEntry(ctx, entry)
}

func (s *FinanceService) PnL(ctx context.Context, restaurantID uint, from, to time.Time) (core.PnLReport, error) {
	categories, totals, plans, rules, err := s.reportData(ctx, restaurantID, from, to)
	if err != nil {
		return core.PnLReport{}, err
	}
	return core.BuildPnLFromTotals(restaurantID, from, to, categories, totals, plans, rules), nil
}

func (s *FinanceService) CashFlow(ctx context.Context, restaurantID uint, from, to time.Time) (core.CashFlowReport, error) {
	var categories []core.FinancialCategory
	var totals map[uint]decimal.Decimal
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		categories, err = s.store.Categories(restaurantID)
		return err
	})
	group.Go(func() error {
		var err error
		totals, err = s.store.EntryTotals(groupCtx, restaurantID, from, to)
		return err
	})
	if err := group.Wait(); err != nil {
		return core.CashFlowReport{}, err
	}
	return core.BuildCashFlowFromTotals(restaurantID, from, to, categories, totals), nil
}

func (s *FinanceService) Payroll(ctx context.Context, restaurantID uint, from, to time.Time) (core.PayrollReport, error) {
	var employees []core.Employee
	var shifts []core.Shift
	var pnl core.PnLReport
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		employees, err = s.store.PayrollEmployees(restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		shifts, err = s.store.Shifts(restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		pnl, err = s.PnL(groupCtx, restaurantID, from, to)
		return err
	})
	if err := group.Wait(); err != nil {
		return core.PayrollReport{}, err
	}
	return core.BuildPayroll(from, to, employees, shifts, pnl.Revenue), nil
}

func (s *FinanceService) BreakEven(ctx context.Context, restaurantID uint, from, to time.Time) (decimal.Decimal, error) {
	pnl, err := s.PnL(ctx, restaurantID, from, to)
	if err != nil {
		return decimal.Zero, err
	}
	return breakEvenFromPnL(pnl), nil
}

func (s *FinanceService) Dashboard(ctx context.Context, restaurantID uint, from, to time.Time) (Dashboard, error) {
	var categories []core.FinancialCategory
	var totals map[uint]decimal.Decimal
	var plans []core.PlanValue
	var rules []core.CalculationRule
	var employees []core.Employee
	var shifts []core.Shift

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		categories, err = s.store.Categories(restaurantID)
		return err
	})
	group.Go(func() error {
		var err error
		totals, err = s.store.EntryTotals(groupCtx, restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		plans, err = s.store.Plans(restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		rules, err = s.store.Rules(restaurantID)
		return err
	})
	group.Go(func() error {
		var err error
		employees, err = s.store.PayrollEmployees(restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		shifts, err = s.store.Shifts(restaurantID, from, to)
		return err
	})
	if err := group.Wait(); err != nil {
		return Dashboard{}, err
	}

	pnl := core.BuildPnLFromTotals(restaurantID, from, to, categories, totals, plans, rules)
	return Dashboard{
		PnL:       pnl,
		CashFlow:  core.BuildCashFlowFromTotals(restaurantID, from, to, categories, totals),
		Payroll:   core.BuildPayroll(from, to, employees, shifts, pnl.Revenue),
		BreakEven: breakEvenFromPnL(pnl),
	}, nil
}

func breakEvenFromPnL(pnl core.PnLReport) decimal.Decimal {
	fixed := pnl.ControlledExpenses.Add(pnl.Payroll).Add(pnl.Uncontrolled).Add(pnl.Overhead)
	ratio := decimal.Zero
	if !pnl.Revenue.IsZero() {
		ratio = pnl.COGS.Div(pnl.Revenue)
	}
	return core.BreakEvenRevenue(fixed, ratio)
}

func (s *FinanceService) ValidateRule(rule *core.CalculationRule) error {
	if rule.RestaurantID == 0 || strings.TrimSpace(rule.Name) == "" {
		return errors.New("restaurant_id and name are required")
	}
	switch rule.RuleType {
	case "classification":
		if rule.TargetCategoryID == nil {
			return errors.New("classification rule requires target_category_id")
		}
		switch rule.MatchField {
		case "description", "counterparty", "payment_method", "source", "direction", "amount":
		default:
			return fmt.Errorf("unsupported match field %q", rule.MatchField)
		}
		if rule.MatchField == "amount" {
			switch rule.MatchOperator {
			case "equals", "greater_than", "greater_or_equal", "less_than", "less_or_equal":
			default:
				return fmt.Errorf("unsupported numeric match operator %q", rule.MatchOperator)
			}
			if _, err := decimal.NewFromString(rule.MatchValue); err != nil {
				return errors.New("amount rule requires a numeric match_value")
			}
		} else {
			switch rule.MatchOperator {
			case "equals", "contains", "starts_with", "ends_with", "not_contains":
			default:
				return fmt.Errorf("unsupported match operator %q", rule.MatchOperator)
			}
		}
	case "calculation":
		if rule.TargetCategoryID == nil {
			return errors.New("calculation rule requires target_category_id")
		}
		if rule.SourceCategoryID == nil && rule.SourceMetric == "" && rule.Operation != "fixed" {
			return errors.New("calculation rule requires source_category_id or source_metric")
		}
		switch rule.Operation {
		case "percent_of", "copy", "fixed":
		default:
			return fmt.Errorf("unsupported operation %q", rule.Operation)
		}
	default:
		return errors.New("rule_type must be classification or calculation")
	}
	for _, categoryID := range []*uint{rule.SourceCategoryID, rule.TargetCategoryID} {
		if categoryID == nil {
			continue
		}
		ok, err := s.store.CategoryBelongsToRestaurant(rule.RestaurantID, *categoryID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("rule category does not belong to restaurant")
		}
	}
	return nil
}

func (s *FinanceService) ValidateCategory(category *core.FinancialCategory) error {
	if category.RestaurantID == 0 || strings.TrimSpace(category.Name) == "" {
		return errors.New("restaurant_id and name are required")
	}
	if !categoryCodePattern.MatchString(category.Code) {
		return errors.New("code must contain lowercase latin letters, digits and underscores")
	}
	switch category.Kind {
	case core.CategoryRevenue, core.CategoryCOGS, core.CategoryControlled, core.CategoryPayroll,
		core.CategoryUncontrolled, core.CategoryOverhead, core.CategoryDepreciation, core.CategoryTax,
		core.CategoryCashIn, core.CategoryCashOut:
	default:
		return errors.New("unsupported category kind")
	}
	if category.Report != "PNL" && category.Report != "DDS" && category.Report != "BOTH" {
		return errors.New("report must be PNL, DDS or BOTH")
	}
	if category.ParentID != nil {
		ok, err := s.store.CategoryBelongsToRestaurant(category.RestaurantID, *category.ParentID)
		if err != nil {
			return err
		}
		if !ok {
			return errors.New("parent category does not belong to restaurant")
		}
	}
	return nil
}

func ValidateEmployee(employee *core.Employee) error {
	switch {
	case employee.RestaurantID == 0:
		return errors.New("сначала выберите или создайте ресторан")
	case strings.TrimSpace(employee.Name) == "":
		return errors.New("employee name is required")
	case strings.TrimSpace(employee.Position) == "":
		return errors.New("employee position is required")
	case employee.HourlyRate.IsNegative() || employee.MonthlyRate.IsNegative():
		return errors.New("employee rates cannot be negative")
	case employee.KPIPercent.IsNegative() || employee.KPIPercent.GreaterThan(decimal.NewFromInt(1)):
		return errors.New("kpi_percent must be between 0 and 1")
	default:
		return nil
	}
}

func (s *FinanceService) ValidateEmployee(employee *core.Employee) error {
	if err := ValidateEmployee(employee); err != nil {
		return err
	}
	exists, err := s.store.RestaurantExists(employee.RestaurantID)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("выбранный ресторан не найден; выберите или создайте ресторан")
	}
	return nil
}

func (s *FinanceService) reportData(ctx context.Context, restaurantID uint, from, to time.Time) ([]core.FinancialCategory, map[uint]decimal.Decimal, []core.PlanValue, []core.CalculationRule, error) {
	var categories []core.FinancialCategory
	var totals map[uint]decimal.Decimal
	var plans []core.PlanValue
	var rules []core.CalculationRule
	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		categories, err = s.store.Categories(restaurantID)
		return err
	})
	group.Go(func() error {
		var err error
		totals, err = s.store.EntryTotals(groupCtx, restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		plans, err = s.store.Plans(restaurantID, from, to)
		return err
	})
	group.Go(func() error {
		var err error
		rules, err = s.store.Rules(restaurantID)
		return err
	})
	if err := group.Wait(); err != nil {
		return nil, nil, nil, nil, err
	}
	return categories, totals, plans, rules, nil
}

func defaultCategories() []core.FinancialCategory {
	return []core.FinancialCategory{
		{Code: "revenue_kitchen", Name: "Выручка — кухня", Kind: core.CategoryRevenue, Report: "BOTH"},
		{Code: "revenue_alcohol", Name: "Выручка — алкоголь", Kind: core.CategoryRevenue, Report: "BOTH"},
		{Code: "revenue_soft", Name: "Выручка — безалкогольные напитки", Kind: core.CategoryRevenue, Report: "BOTH"},
		{Code: "revenue_hookah", Name: "Выручка — кальяны", Kind: core.CategoryRevenue, Report: "BOTH"},
		{Code: "revenue_other", Name: "Выручка — прочее", Kind: core.CategoryRevenue, Report: "BOTH"},
		{Code: "cogs_kitchen", Name: "Себестоимость — кухня", Kind: core.CategoryCOGS, Report: "PNL"},
		{Code: "cogs_alcohol", Name: "Себестоимость — алкоголь", Kind: core.CategoryCOGS, Report: "PNL"},
		{Code: "cogs_soft", Name: "Себестоимость — безалкогольные напитки", Kind: core.CategoryCOGS, Report: "PNL"},
		{Code: "cogs_hookah", Name: "Себестоимость — кальяны", Kind: core.CategoryCOGS, Report: "PNL"},
		{Code: "payroll", Name: "Оплата труда", Kind: core.CategoryPayroll, Report: "BOTH"},
		{Code: "utilities", Name: "Коммунальные услуги", Kind: core.CategoryControlled, Report: "BOTH"},
		{Code: "services", Name: "Другие услуги", Kind: core.CategoryControlled, Report: "BOTH"},
		{Code: "materials", Name: "Материалы и хозяйственные расходы", Kind: core.CategoryControlled, Report: "BOTH"},
		{Code: "marketing", Name: "Маркетинг", Kind: core.CategoryControlled, Report: "BOTH"},
		{Code: "acquiring", Name: "Эквайринг", Kind: core.CategoryControlled, Report: "BOTH"},
		{Code: "rent", Name: "Аренда", Kind: core.CategoryUncontrolled, Report: "BOTH"},
		{Code: "management", Name: "Управляющая компания", Kind: core.CategoryUncontrolled, Report: "BOTH"},
		{Code: "overhead", Name: "Накладные расходы", Kind: core.CategoryOverhead, Report: "BOTH"},
		{Code: "depreciation", Name: "Амортизация", Kind: core.CategoryDepreciation, Report: "PNL"},
		{Code: "tax", Name: "Налоги", Kind: core.CategoryTax, Report: "BOTH"},
		{Code: "cash_in_other", Name: "Прочие поступления", Kind: core.CategoryCashIn, Report: "DDS"},
		{Code: "cash_out_other", Name: "Прочие выплаты", Kind: core.CategoryCashOut, Report: "DDS"},
	}
}
