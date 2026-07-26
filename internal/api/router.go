package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"github.com/yourusername/restaurant-finance/internal/adapter/excel"
	"github.com/yourusername/restaurant-finance/internal/core"
	"github.com/yourusername/restaurant-finance/internal/repository"
	"github.com/yourusername/restaurant-finance/internal/service"
	appweb "github.com/yourusername/restaurant-finance/web"
)

type API struct {
	finance *service.FinanceService
	excel   *service.ExcelService
	pos     *service.POSService
}

const (
	maxJSONBody  = 2 << 20
	maxExcelBody = 50 << 20
)

func NewRouter(finance *service.FinanceService, excelService *service.ExcelService, pos *service.POSService, allowedOrigin string) *gin.Engine {
	api := &API{finance: finance, excel: excelService, pos: pos}
	router := gin.Default()
	router.MaxMultipartMemory = 8 << 20
	_ = router.SetTrustedProxies(nil)
	router.Use(cors(allowedOrigin))
	router.Use(securityHeaders())
	router.GET("/ping", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	v1 := router.Group("/api/v1")
	{
		v1.GET("/restaurants", api.restaurants)
		v1.POST("/restaurants", api.createRestaurant)
		v1.GET("/restaurants/:id/categories", api.categories)
		v1.POST("/restaurants/:id/categories", api.createCategory)
		v1.GET("/restaurants/:id/entries", api.entries)
		v1.POST("/restaurants/:id/entries", api.createEntry)
		v1.PUT("/restaurants/:id/entries/:entryID", api.updateEntry)
		v1.DELETE("/restaurants/:id/entries/:entryID", api.deleteEntry)
		v1.POST("/restaurants/:id/plans", api.savePlan)
		v1.GET("/restaurants/:id/rules", api.rules)
		v1.POST("/restaurants/:id/rules", api.createRule)
		v1.PUT("/restaurants/:id/rules/:ruleID", api.updateRule)
		v1.GET("/restaurants/:id/reports/pnl", api.pnl)
		v1.GET("/restaurants/:id/reports/dashboard", api.dashboard)
		v1.GET("/restaurants/:id/reports/cash-flow", api.cashFlow)
		v1.GET("/restaurants/:id/reports/payroll", api.payroll)
		v1.GET("/restaurants/:id/reports/break-even", api.breakEven)
		v1.POST("/restaurants/:id/employees", api.saveEmployee)
		v1.GET("/restaurants/:id/employees", api.employees)
		v1.DELETE("/restaurants/:id/employees/:employeeID", api.deleteEmployee)
		v1.GET("/restaurants/:id/shifts", api.shifts)
		v1.POST("/restaurants/:id/shifts", api.saveShift)
		v1.POST("/restaurants/:id/pos-connections", api.savePOSConnection)
		v1.GET("/restaurants/:id/pos-connections", api.posConnections)
		v1.POST("/restaurants/:id/pos-connections/:connectionID/test", api.testPOS)
		v1.POST("/restaurants/:id/pos-connections/:connectionID/sync", api.syncPOS)
		v1.POST("/excel/preview", api.previewExcel)
		v1.POST("/restaurants/:id/excel/import", api.importExcel)
		v1.GET("/restaurants/:id/excel/export", api.exportExcel)
		v1.GET("/restaurants/:id/reports/payroll/export", api.exportPayroll)
	}
	assets, err := fs.Sub(appweb.Static, "static")
	if err != nil {
		panic(err)
	}
	router.StaticFS("/app", http.FS(assets))
	router.GET("/", func(c *gin.Context) { c.Redirect(http.StatusTemporaryRedirect, "/app/") })
	return router
}

func (a *API) restaurants(c *gin.Context) {
	values, err := a.finance.Store().Restaurants()
	respond(c, values, err)
}

func (a *API) createRestaurant(c *gin.Context) {
	var value core.Restaurant
	if !bind(c, &value) {
		return
	}
	if value.Name == "" {
		badRequest(c, errors.New("name is required"))
		return
	}
	if value.Currency == "" {
		value.Currency = "RUB"
	}
	if value.Timezone == "" {
		value.Timezone = "Europe/Moscow"
	}
	err := a.finance.CreateRestaurant(c.Request.Context(), &value)
	respondCreated(c, value, err)
}

func (a *API) categories(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	values, err := a.finance.Store().Categories(id)
	respond(c, values, err)
}

func (a *API) createCategory(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	var value core.FinancialCategory
	if !bind(c, &value) {
		return
	}
	value.RestaurantID = restaurantID
	if value.Report == "" {
		value.Report = "BOTH"
	}
	value.Active = true
	if err := a.finance.ValidateCategory(&value); err != nil {
		badRequest(c, err)
		return
	}
	err := a.finance.Store().CreateCategory(&value)
	respondCreated(c, value, err)
}

func (a *API) entries(c *gin.Context) {
	restaurantID, from, to, ok := period(c)
	if !ok {
		return
	}
	filter := repository.EntryFilter{
		RestaurantID: restaurantID,
		From:         from,
		To:           to,
		Direction:    c.Query("direction"),
		Query:        c.Query("q"),
		Limit:        queryInt(c, "limit", 200),
		Offset:       queryInt(c, "offset", 0),
	}
	if raw := c.Query("category_id"); raw != "" {
		value, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			badRequest(c, errors.New("category_id must be an integer"))
			return
		}
		categoryID := uint(value)
		filter.CategoryID = &categoryID
	}
	values, err := a.finance.Store().Entries(c.Request.Context(), filter)
	respond(c, values, err)
}

func (a *API) createEntry(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	var request struct {
		CategoryID    *uint  `json:"category_id"`
		Date          string `json:"date"`
		Amount        string `json:"amount"`
		Direction     string `json:"direction"`
		PaymentMethod string `json:"payment_method"`
		Description   string `json:"description"`
		Counterparty  string `json:"counterparty"`
		Tags          string `json:"tags"`
	}
	if !bind(c, &request) {
		return
	}
	date, err := parseDate(request.Date)
	if err != nil {
		badRequest(c, err)
		return
	}
	amount, err := decimalFromString(request.Amount)
	if err != nil {
		badRequest(c, err)
		return
	}
	if amount.IsNegative() {
		badRequest(c, errors.New("plan amount cannot be negative"))
		return
	}
	value := core.FinancialEntry{
		RestaurantID: restaurantID, CategoryID: request.CategoryID, OccurredAt: date,
		Amount: amount, Direction: request.Direction, PaymentMethod: request.PaymentMethod,
		Description: request.Description, Counterparty: request.Counterparty,
		Tags: request.Tags,
	}
	err = a.finance.SaveManualEntry(c.Request.Context(), &value)
	respondCreated(c, value, err)
}

func (a *API) updateEntry(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	entryID, ok := pathID(c, "entryID")
	if !ok {
		return
	}
	var request struct {
		CategoryID    *uint  `json:"category_id"`
		Date          string `json:"date"`
		Amount        string `json:"amount"`
		Direction     string `json:"direction"`
		PaymentMethod string `json:"payment_method"`
		Description   string `json:"description"`
		Counterparty  string `json:"counterparty"`
		Tags          string `json:"tags"`
	}
	if !bind(c, &request) {
		return
	}
	date, err := parseDate(request.Date)
	if err != nil {
		badRequest(c, err)
		return
	}
	amount, err := decimalFromString(request.Amount)
	if err != nil || amount.IsNegative() {
		badRequest(c, errors.New("amount must be a non-negative number"))
		return
	}
	value := core.FinancialEntry{ID: entryID, RestaurantID: restaurantID, CategoryID: request.CategoryID, OccurredAt: date, Amount: amount, Direction: request.Direction, PaymentMethod: request.PaymentMethod, Description: request.Description, Counterparty: request.Counterparty, Tags: request.Tags}
	err = a.finance.UpdateManualEntry(c.Request.Context(), &value)
	if repository.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	respond(c, value, err)
}

func (a *API) deleteEntry(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	entryID, ok := pathID(c, "entryID")
	if !ok {
		return
	}
	err := a.finance.Store().DeleteEntry(restaurantID, entryID)
	if repository.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}
	respond(c, gin.H{"deleted": entryID}, err)
}

func (a *API) savePlan(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	var request struct {
		CategoryID uint   `json:"category_id"`
		Month      string `json:"month"`
		Amount     string `json:"amount"`
	}
	if !bind(c, &request) {
		return
	}
	month, err := time.Parse("2006-01", request.Month)
	if err != nil {
		badRequest(c, errors.New("month must use YYYY-MM"))
		return
	}
	amount, err := decimalFromString(request.Amount)
	if err != nil {
		badRequest(c, err)
		return
	}
	belongs, err := a.finance.Store().CategoryBelongsToRestaurant(restaurantID, request.CategoryID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	if !belongs {
		badRequest(c, errors.New("category does not belong to restaurant"))
		return
	}
	value := core.PlanValue{RestaurantID: restaurantID, CategoryID: request.CategoryID, Month: month, Amount: amount}
	err = a.finance.Store().UpsertPlan(&value)
	respondCreated(c, value, err)
}

func (a *API) rules(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	values, err := a.finance.Store().Rules(id)
	respond(c, values, err)
}

func (a *API) createRule(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	var value core.CalculationRule
	if !bind(c, &value) {
		return
	}
	value.RestaurantID = restaurantID
	value.Active = true
	if err := a.finance.ValidateRule(&value); err != nil {
		badRequest(c, err)
		return
	}
	err := a.finance.Store().CreateRule(&value)
	respondCreated(c, value, err)
}

func (a *API) updateRule(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	ruleID, ok := pathID(c, "ruleID")
	if !ok {
		return
	}
	var value core.CalculationRule
	if !bind(c, &value) {
		return
	}
	value.ID, value.RestaurantID = ruleID, restaurantID
	if err := a.finance.ValidateRule(&value); err != nil {
		badRequest(c, err)
		return
	}
	err := a.finance.Store().UpdateRule(&value)
	respond(c, value, err)
}

func (a *API) pnl(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	value, err := a.finance.PnL(c.Request.Context(), id, from, to)
	respond(c, value, err)
}

func (a *API) dashboard(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	value, err := a.finance.Dashboard(c.Request.Context(), id, from, to)
	respond(c, value, err)
}

func (a *API) cashFlow(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	value, err := a.finance.CashFlow(c.Request.Context(), id, from, to)
	respond(c, value, err)
}

func (a *API) payroll(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	value, err := a.finance.Payroll(c.Request.Context(), id, from, to)
	respond(c, value, err)
}

func (a *API) breakEven(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	value, err := a.finance.BreakEven(c.Request.Context(), id, from, to)
	respond(c, gin.H{"break_even_revenue": value}, err)
}

func (a *API) saveEmployee(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var value core.Employee
	if !bind(c, &value) {
		return
	}
	value.RestaurantID, value.Active = id, true
	if err := a.finance.ValidateEmployee(&value); err != nil {
		badRequest(c, err)
		return
	}
	err := a.finance.Store().SaveEmployee(&value)
	respondCreated(c, value, err)
}

func (a *API) employees(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var values []core.Employee
	var err error
	if c.Query("include_inactive") == "true" {
		values, err = a.finance.Store().AllEmployees(id)
	} else {
		values, err = a.finance.Store().Employees(id)
	}
	respond(c, values, err)
}

func (a *API) deleteEmployee(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	employeeID, ok := pathID(c, "employeeID")
	if !ok {
		return
	}
	err := a.finance.Store().DeactivateEmployee(restaurantID, employeeID)
	if repository.IsNotFound(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "employee not found"})
		return
	}
	respond(c, gin.H{"deleted": employeeID}, err)
}

func (a *API) saveShift(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	var request struct {
		ID         uint   `json:"id"`
		EmployeeID uint   `json:"employee_id"`
		Date       string `json:"date"`
		Hours      string `json:"hours"`
		Bonus      string `json:"bonus"`
		Advance    string `json:"advance"`
		Deduction  string `json:"deduction"`
		Comment    string `json:"comment"`
	}
	if !bind(c, &request) {
		return
	}
	belongs, err := a.finance.Store().EmployeeBelongsToRestaurant(restaurantID, request.EmployeeID)
	if err != nil {
		respond(c, nil, err)
		return
	}
	if !belongs {
		badRequest(c, errors.New("employee does not belong to restaurant"))
		return
	}
	date, err := parseDate(request.Date)
	if err != nil {
		badRequest(c, errors.New("date must use YYYY-MM-DD"))
		return
	}
	hours, err := decimalFromDefault(request.Hours)
	if err != nil {
		badRequest(c, err)
		return
	}
	bonus, err := decimalFromDefault(request.Bonus)
	if err != nil {
		badRequest(c, err)
		return
	}
	advance, err := decimalFromDefault(request.Advance)
	if err != nil {
		badRequest(c, err)
		return
	}
	deduction, err := decimalFromDefault(request.Deduction)
	if err != nil {
		badRequest(c, err)
		return
	}
	if hours.LessThanOrEqual(decimal.Zero) || hours.GreaterThan(decimal.NewFromInt(24)) {
		badRequest(c, errors.New("hours must be greater than 0 and no more than 24"))
		return
	}
	if bonus.IsNegative() || advance.IsNegative() || deduction.IsNegative() {
		badRequest(c, errors.New("bonus, advance and deduction cannot be negative"))
		return
	}
	value := core.Shift{
		ID: request.ID, EmployeeID: request.EmployeeID, Date: date, Hours: hours,
		Bonus: bonus, Advance: advance, Deduction: deduction, Comment: request.Comment,
	}
	err = a.finance.Store().SaveShift(&value)
	respondCreated(c, value, err)
}

func (a *API) shifts(c *gin.Context) {
	id, from, to, ok := period(c)
	if !ok {
		return
	}
	values, err := a.finance.Store().Shifts(id, from, to)
	respond(c, values, err)
}

func (a *API) savePOSConnection(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	var request struct {
		ID       uint   `json:"id"`
		Provider string `json:"provider"`
		Name     string `json:"name"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Settings string `json:"settings"`
		Active   bool   `json:"active"`
	}
	if !bind(c, &request) {
		return
	}
	if request.Name == "" || request.Provider == "" || request.BaseURL == "" {
		badRequest(c, errors.New("name, provider and base_url are required"))
		return
	}
	if !a.pos.Supports(request.Provider) {
		badRequest(c, errors.New("unsupported POS provider"))
		return
	}
	value := core.POSConnection{
		ID: request.ID, RestaurantID: id, Provider: request.Provider, Name: request.Name,
		BaseURL: request.BaseURL, APIKey: request.APIKey, Settings: request.Settings, Active: request.Active,
	}
	err := a.finance.Store().SavePOSConnection(&value)
	respondCreated(c, value, err)
}

func (a *API) posConnections(c *gin.Context) {
	id, ok := pathID(c, "id")
	if !ok {
		return
	}
	values, err := a.finance.Store().POSConnections(id)
	respond(c, values, err)
}

func (a *API) syncPOS(c *gin.Context) {
	restaurantID, from, to, ok := period(c)
	if !ok {
		return
	}
	connectionID, ok := pathID(c, "connectionID")
	if !ok {
		return
	}
	count, err := a.pos.Sync(c.Request.Context(), restaurantID, connectionID, from, to)
	respond(c, gin.H{"imported": count}, err)
}

func (a *API) testPOS(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	connectionID, ok := pathID(c, "connectionID")
	if !ok {
		return
	}
	err := a.pos.Test(c.Request.Context(), restaurantID, connectionID)
	respond(c, gin.H{"status": "ok"}, err)
}

func (a *API) previewExcel(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxExcelBody)
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, err)
		return
	}
	opened, err := file.Open()
	if err != nil {
		respond(c, nil, err)
		return
	}
	defer opened.Close()
	value, err := a.excel.Preview(opened)
	respond(c, value, err)
}

func (a *API) importExcel(c *gin.Context) {
	restaurantID, ok := pathID(c, "id")
	if !ok {
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxExcelBody)
	file, err := c.FormFile("file")
	if err != nil {
		badRequest(c, err)
		return
	}
	opened, err := file.Open()
	if err != nil {
		respond(c, nil, err)
		return
	}
	defer opened.Close()
	var result excel.ImportResult
	if c.PostForm("mode") == "template" {
		month, parseErr := time.Parse("2006-01", c.PostForm("month"))
		if parseErr != nil {
			badRequest(c, errors.New("month must use YYYY-MM"))
			return
		}
		result, err = a.excel.ImportTemplate(c.Request.Context(), opened, restaurantID, month)
	} else {
		var mapping excel.ColumnMapping
		if parseErr := json.Unmarshal([]byte(c.PostForm("mapping")), &mapping); parseErr != nil {
			badRequest(c, errors.New("mapping must be valid JSON"))
			return
		}
		result, err = a.excel.Import(c.Request.Context(), opened, restaurantID, mapping)
	}
	respond(c, result, err)
}

func (a *API) exportExcel(c *gin.Context) {
	restaurantID, from, to, ok := period(c)
	if !ok {
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="restaurant-finance.xlsx"`)
	if err := a.excel.Export(c.Request.Context(), c.Writer, restaurantID, from, to); err != nil {
		_ = c.Error(err)
	}
}

func (a *API) exportPayroll(c *gin.Context) {
	restaurantID, from, to, ok := period(c)
	if !ok {
		return
	}
	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Header("Content-Disposition", `attachment; filename="payroll.xlsx"`)
	if err := a.excel.ExportPayroll(c.Request.Context(), c.Writer, restaurantID, from, to); err != nil {
		_ = c.Error(err)
	}
}

func respond(c *gin.Context, value any, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, value)
}

func respondCreated(c *gin.Context, value any, err error) {
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, value)
}

func bind(c *gin.Context, value any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxJSONBody)
	if err := c.ShouldBindJSON(value); err != nil {
		badRequest(c, err)
		return false
	}
	return true
}

func badRequest(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func pathID(c *gin.Context, name string) (uint, bool) {
	value, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || value == 0 {
		badRequest(c, fmtError(name+" must be a positive integer"))
		return 0, false
	}
	return uint(value), true
}

func period(c *gin.Context) (uint, time.Time, time.Time, bool) {
	id, ok := pathID(c, "id")
	if !ok {
		return 0, time.Time{}, time.Time{}, false
	}
	from, err := parseDate(c.Query("from"))
	if err != nil {
		badRequest(c, errors.New("from must use YYYY-MM-DD"))
		return 0, time.Time{}, time.Time{}, false
	}
	to, err := parseDate(c.Query("to"))
	if err != nil || to.Before(from) {
		badRequest(c, errors.New("to must use YYYY-MM-DD and not precede from"))
		return 0, time.Time{}, time.Time{}, false
	}
	return id, from, to, true
}

func parseDate(value string) (time.Time, error) {
	return time.Parse("2006-01-02", value)
}

func decimalFromString(value string) (decimal.Decimal, error) {
	return decimal.NewFromString(value)
}

func decimalFromDefault(value string) (decimal.Decimal, error) {
	if value == "" {
		return decimal.Zero, nil
	}
	return decimal.NewFromString(value)
}

func queryInt(c *gin.Context, name string, fallback int) int {
	value, err := strconv.Atoi(c.Query(name))
	if err != nil {
		return fallback
	}
	return value
}

func fmtError(message string) error { return errors.New(message) }

func errorStatus(err error) int {
	if repository.IsNotFound(err) {
		return http.StatusNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return http.StatusConflict
		case "23503", "23514":
			return http.StatusBadRequest
		}
	}
	return http.StatusInternalServerError
}

func writeError(c *gin.Context, err error) {
	status := errorStatus(err)
	message := err.Error()
	if status >= http.StatusInternalServerError {
		message = "internal server error"
		slog.Error("request failed", "method", c.Request.Method, "path", c.Request.URL.Path, "error", err)
	}
	c.JSON(status, gin.H{"error": message})
}

func cors(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Content-Security-Policy", "default-src 'self'; connect-src 'self' http://127.0.0.1:18080 http://localhost:8080; img-src 'self' data:; style-src 'self'; script-src 'self'")
		c.Next()
	}
}
