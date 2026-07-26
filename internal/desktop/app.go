package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/yourusername/restaurant-finance/internal/adapter/excel"
	"github.com/yourusername/restaurant-finance/internal/bootstrap"
	"github.com/yourusername/restaurant-finance/internal/core"
	"github.com/yourusername/restaurant-finance/internal/repository"
	"github.com/yourusername/restaurant-finance/internal/service"
)

const maxExcelSize = 50 << 20

type App struct {
	ctx      context.Context
	services *bootstrap.Services
	dbPath   string
}

type Period struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type EntryInput struct {
	CategoryID    *uint  `json:"category_id"`
	Date          string `json:"date"`
	Amount        string `json:"amount"`
	Direction     string `json:"direction"`
	PaymentMethod string `json:"payment_method"`
	Description   string `json:"description"`
	Counterparty  string `json:"counterparty"`
	Tags          string `json:"tags"`
}

type EntryQuery struct {
	CategoryID *uint  `json:"category_id"`
	Direction  string `json:"direction"`
	Query      string `json:"query"`
	Limit      int    `json:"limit"`
	Offset     int    `json:"offset"`
}

type PlanInput struct {
	CategoryID uint   `json:"category_id"`
	Month      string `json:"month"`
	Amount     string `json:"amount"`
}

type ShiftInput struct {
	ID         uint   `json:"id"`
	EmployeeID uint   `json:"employee_id"`
	Date       string `json:"date"`
	Hours      string `json:"hours"`
	Bonus      string `json:"bonus"`
	Advance    string `json:"advance"`
	Deduction  string `json:"deduction"`
	Comment    string `json:"comment"`
}

type POSConnectionInput struct {
	ID       uint   `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Settings string `json:"settings"`
	Active   bool   `json:"active"`
}

type ExcelSelection struct {
	Path    string        `json:"path"`
	Preview excel.Preview `json:"preview"`
}

type ExcelImportInput struct {
	Path         string              `json:"path"`
	RestaurantID uint                `json:"restaurant_id"`
	TemplateMode bool                `json:"template_mode"`
	Month        string              `json:"month"`
	Mapping      excel.ColumnMapping `json:"mapping"`
}

type DatabaseInfo struct {
	Driver string `json:"driver"`
	Path   string `json:"path"`
}

func New(services *bootstrap.Services, dbPath string) *App {
	return &App{services: services, dbPath: dbPath}
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) Shutdown(context.Context) {
	sqlDB, err := a.services.Store.DB.DB()
	if err == nil {
		_ = sqlDB.Close()
	}
}

func (a *App) DatabaseInfo() DatabaseInfo {
	return DatabaseInfo{Driver: "sqlite", Path: a.dbPath}
}

func (a *App) Restaurants() ([]core.Restaurant, error) {
	return a.services.Store.Restaurants()
}

func (a *App) CreateRestaurant(value core.Restaurant) (core.Restaurant, error) {
	if strings.TrimSpace(value.Name) == "" {
		return value, errors.New("name is required")
	}
	if value.Currency == "" {
		value.Currency = "RUB"
	}
	if value.Timezone == "" {
		value.Timezone = "Europe/Moscow"
	}
	return value, a.services.Finance.CreateRestaurant(a.context(), &value)
}

func (a *App) Categories(restaurantID uint) ([]core.FinancialCategory, error) {
	return a.services.Store.Categories(restaurantID)
}

func (a *App) CreateCategory(restaurantID uint, value core.FinancialCategory) (core.FinancialCategory, error) {
	value.RestaurantID = restaurantID
	if value.Report == "" {
		value.Report = "BOTH"
	}
	value.Active = true
	if err := a.services.Finance.ValidateCategory(&value); err != nil {
		return value, err
	}
	return value, a.services.Store.CreateCategory(&value)
}

func (a *App) Entries(restaurantID uint, period Period, filter EntryQuery) ([]core.FinancialEntry, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return nil, err
	}
	return a.services.Store.Entries(a.context(), repository.EntryFilter{
		RestaurantID: restaurantID,
		From:         from,
		To:           to,
		CategoryID:   filter.CategoryID,
		Direction:    filter.Direction,
		Query:        filter.Query,
		Limit:        filter.Limit,
		Offset:       filter.Offset,
	})
}

func (a *App) CreateEntry(restaurantID uint, input EntryInput) (core.FinancialEntry, error) {
	date, err := parseDate(input.Date)
	if err != nil {
		return core.FinancialEntry{}, err
	}
	amount, err := positiveDecimal(input.Amount, "amount")
	if err != nil {
		return core.FinancialEntry{}, err
	}
	value := core.FinancialEntry{
		RestaurantID:  restaurantID,
		CategoryID:    input.CategoryID,
		OccurredAt:    date,
		Amount:        amount,
		Direction:     input.Direction,
		PaymentMethod: input.PaymentMethod,
		Description:   input.Description,
		Counterparty:  input.Counterparty,
		Tags:          input.Tags,
	}
	return value, a.services.Finance.SaveManualEntry(a.context(), &value)
}

func (a *App) UpdateEntry(restaurantID, entryID uint, input EntryInput) (core.FinancialEntry, error) {
	date, err := parseDate(input.Date)
	if err != nil {
		return core.FinancialEntry{}, err
	}
	amount, err := positiveDecimal(input.Amount, "amount")
	if err != nil {
		return core.FinancialEntry{}, err
	}
	value := core.FinancialEntry{
		ID: entryID, RestaurantID: restaurantID, CategoryID: input.CategoryID,
		OccurredAt: date, Amount: amount, Direction: input.Direction,
		PaymentMethod: input.PaymentMethod, Description: input.Description,
		Counterparty: input.Counterparty, Tags: input.Tags,
	}
	return value, a.services.Finance.UpdateManualEntry(a.context(), &value)
}

func (a *App) SavePlan(restaurantID uint, input PlanInput) (core.PlanValue, error) {
	month, err := time.Parse("2006-01", input.Month)
	if err != nil {
		return core.PlanValue{}, errors.New("month must use YYYY-MM")
	}
	amount, err := decimal.NewFromString(input.Amount)
	if err != nil || amount.IsNegative() {
		return core.PlanValue{}, errors.New("plan amount must be a non-negative number")
	}
	belongs, err := a.services.Store.CategoryBelongsToRestaurant(restaurantID, input.CategoryID)
	if err != nil {
		return core.PlanValue{}, err
	}
	if !belongs {
		return core.PlanValue{}, errors.New("category does not belong to restaurant")
	}
	value := core.PlanValue{
		RestaurantID: restaurantID,
		CategoryID:   input.CategoryID,
		Month:        month,
		Amount:       amount,
	}
	return value, a.services.Store.UpsertPlan(&value)
}

func (a *App) Rules(restaurantID uint) ([]core.CalculationRule, error) {
	return a.services.Store.Rules(restaurantID)
}

func (a *App) CreateRule(restaurantID uint, value core.CalculationRule) (core.CalculationRule, error) {
	value.RestaurantID = restaurantID
	value.Active = true
	if err := a.services.Finance.ValidateRule(&value); err != nil {
		return value, err
	}
	return value, a.services.Store.CreateRule(&value)
}

func (a *App) Dashboard(restaurantID uint, period Period) (service.Dashboard, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return service.Dashboard{}, err
	}
	return a.services.Finance.Dashboard(a.context(), restaurantID, from, to)
}

func (a *App) Employees(restaurantID uint) ([]core.Employee, error) {
	return a.services.Store.Employees(restaurantID)
}

func (a *App) InactiveEmployees(restaurantID uint) ([]core.Employee, error) {
	return a.services.Store.InactiveEmployees(restaurantID)
}

func (a *App) SaveEmployee(restaurantID uint, value core.Employee) (core.Employee, error) {
	value.RestaurantID = restaurantID
	value.Active = true
	if err := a.services.Finance.ValidateEmployee(&value); err != nil {
		return value, err
	}
	return value, a.services.Store.SaveEmployee(&value)
}

func (a *App) DeleteEmployee(restaurantID, employeeID uint) error {
	return a.services.Store.DeactivateEmployee(restaurantID, employeeID)
}

func (a *App) SaveShift(restaurantID uint, input ShiftInput) (core.Shift, error) {
	belongs, err := a.services.Store.EmployeeBelongsToRestaurant(restaurantID, input.EmployeeID)
	if err != nil {
		return core.Shift{}, err
	}
	if !belongs {
		return core.Shift{}, errors.New("employee does not belong to restaurant")
	}
	date, err := parseDate(input.Date)
	if err != nil {
		return core.Shift{}, err
	}
	hours, err := decimalOrZero(input.Hours)
	if err != nil || hours.LessThanOrEqual(decimal.Zero) || hours.GreaterThan(decimal.NewFromInt(24)) {
		return core.Shift{}, errors.New("hours must be greater than 0 and no more than 24")
	}
	bonus, err := nonNegativeDecimal(input.Bonus, "bonus")
	if err != nil {
		return core.Shift{}, err
	}
	advance, err := nonNegativeDecimal(input.Advance, "advance")
	if err != nil {
		return core.Shift{}, err
	}
	deduction, err := nonNegativeDecimal(input.Deduction, "deduction")
	if err != nil {
		return core.Shift{}, err
	}
	value := core.Shift{
		ID:         input.ID,
		EmployeeID: input.EmployeeID,
		Date:       date,
		Hours:      hours,
		Bonus:      bonus,
		Advance:    advance,
		Deduction:  deduction,
		Comment:    input.Comment,
	}
	return value, a.services.Store.SaveShift(&value)
}

func (a *App) Shifts(restaurantID uint, period Period) ([]core.Shift, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return nil, err
	}
	return a.services.Store.Shifts(restaurantID, from, to)
}

func (a *App) POSConnections(restaurantID uint) ([]core.POSConnection, error) {
	return a.services.Store.POSConnections(restaurantID)
}

func (a *App) SavePOSConnection(restaurantID uint, input POSConnectionInput) (core.POSConnection, error) {
	if strings.TrimSpace(input.Name) == "" || input.Provider == "" || input.BaseURL == "" {
		return core.POSConnection{}, errors.New("name, provider and base_url are required")
	}
	if !a.services.POS.Supports(input.Provider) {
		return core.POSConnection{}, errors.New("unsupported POS provider")
	}
	value := core.POSConnection{
		ID:           input.ID,
		RestaurantID: restaurantID,
		Provider:     input.Provider,
		Name:         input.Name,
		BaseURL:      input.BaseURL,
		APIKey:       input.APIKey,
		Settings:     input.Settings,
		Active:       input.Active,
	}
	return value, a.services.Store.SavePOSConnection(&value)
}

func (a *App) TestPOS(restaurantID, connectionID uint) error {
	return a.services.POS.Test(a.context(), restaurantID, connectionID)
}

func (a *App) SyncPOS(restaurantID, connectionID uint, period Period) (map[string]int, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return nil, err
	}
	count, err := a.services.POS.Sync(a.context(), restaurantID, connectionID, from, to)
	return map[string]int{"imported": count}, err
}

func (a *App) SelectExcel() (ExcelSelection, error) {
	path, err := runtime.OpenFileDialog(a.context(), runtime.OpenDialogOptions{
		Title: "Выберите Excel-файл",
		Filters: []runtime.FileFilter{{
			DisplayName: "Excel (*.xlsx;*.xlsm)",
			Pattern:     "*.xlsx;*.xlsm",
		}},
	})
	if err != nil || path == "" {
		return ExcelSelection{}, err
	}
	file, err := openExcel(path)
	if err != nil {
		return ExcelSelection{}, err
	}
	defer file.Close()
	preview, err := a.services.Excel.Preview(file)
	return ExcelSelection{Path: path, Preview: preview}, err
}

func (a *App) ImportExcel(input ExcelImportInput) (excel.ImportResult, error) {
	file, err := openExcel(input.Path)
	if err != nil {
		return excel.ImportResult{}, err
	}
	defer file.Close()
	if input.TemplateMode {
		month, parseErr := time.Parse("2006-01", input.Month)
		if parseErr != nil {
			return excel.ImportResult{}, errors.New("month must use YYYY-MM")
		}
		return a.services.Excel.ImportTemplate(a.context(), file, input.RestaurantID, month)
	}
	return a.services.Excel.Import(a.context(), file, input.RestaurantID, input.Mapping)
}

func (a *App) ExportExcel(restaurantID uint, period Period) (string, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{
		Title:           "Сохранить финансовый отчёт",
		DefaultFilename: fmt.Sprintf("restaurant-finance-%s-%s.xlsx", period.From, period.To),
		Filters: []runtime.FileFilter{{
			DisplayName: "Excel (*.xlsx)",
			Pattern:     "*.xlsx",
		}},
	})
	if err != nil || path == "" {
		return path, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := a.services.Excel.Export(a.context(), file, restaurantID, from, to); err != nil {
		_ = file.Close()
		return "", err
	}
	return path, file.Close()
}

func (a *App) ExportPayrollExcel(restaurantID uint, period Period) (string, error) {
	from, to, err := parsePeriod(period)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.context(), runtime.SaveDialogOptions{
		Title:           "Сохранить расчёт зарплаты",
		DefaultFilename: fmt.Sprintf("payroll-%s-%s.xlsx", period.From, period.To),
		Filters:         []runtime.FileFilter{{DisplayName: "Excel (*.xlsx)", Pattern: "*.xlsx"}},
	})
	if err != nil || path == "" {
		return path, err
	}
	if !strings.EqualFold(filepath.Ext(path), ".xlsx") {
		path += ".xlsx"
	}
	file, err := os.Create(path)
	if err != nil {
		return "", err
	}
	if err := a.services.Excel.ExportPayroll(a.context(), file, restaurantID, from, to); err != nil {
		_ = file.Close()
		return "", err
	}
	return path, file.Close()
}

func (a *App) context() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func parseDate(value string) (time.Time, error) {
	date, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, errors.New("date must use YYYY-MM-DD")
	}
	return date, nil
}

func parsePeriod(period Period) (time.Time, time.Time, error) {
	from, err := parseDate(period.From)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("from must use YYYY-MM-DD")
	}
	to, err := parseDate(period.To)
	if err != nil || to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to must use YYYY-MM-DD and not precede from")
	}
	return from, to, nil
}

func positiveDecimal(value, name string) (decimal.Decimal, error) {
	number, err := decimal.NewFromString(value)
	if err != nil || number.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, fmt.Errorf("%s must be greater than zero", name)
	}
	return number, nil
}

func nonNegativeDecimal(value, name string) (decimal.Decimal, error) {
	number, err := decimalOrZero(value)
	if err != nil || number.IsNegative() {
		return decimal.Zero, fmt.Errorf("%s cannot be negative", name)
	}
	return number, nil
}

func decimalOrZero(value string) (decimal.Decimal, error) {
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func openExcel(path string) (*os.File, error) {
	extension := strings.ToLower(filepath.Ext(path))
	if extension != ".xlsx" && extension != ".xlsm" {
		return nil, errors.New("only .xlsx and .xlsm files are supported")
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxExcelSize {
		return nil, errors.New("Excel file is larger than 50 MB")
	}
	return os.Open(path)
}
