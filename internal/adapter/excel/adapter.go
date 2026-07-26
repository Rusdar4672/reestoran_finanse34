package excel

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"github.com/yourusername/restaurant-finance/internal/core"
)

type Preview struct {
	Sheets []SheetPreview `json:"sheets"`
}

type SheetPreview struct {
	Name string     `json:"name"`
	Rows [][]string `json:"rows"`
}

type ColumnMapping struct {
	Sheet         string `json:"sheet"`
	HasHeader     bool   `json:"has_header"`
	Date          int    `json:"date"`
	Amount        int    `json:"amount"`
	Direction     int    `json:"direction"`
	Description   int    `json:"description"`
	Counterparty  int    `json:"counterparty"`
	PaymentMethod int    `json:"payment_method"`
	CategoryCode  int    `json:"category_code"`
}

type ImportResult struct {
	Entries  []core.FinancialEntry `json:"entries"`
	Errors   []string              `json:"errors"`
	Imported int                   `json:"imported"`
	Skipped  int                   `json:"skipped"`
}

func ImportRestaurantTemplate(reader io.Reader, restaurantID uint, month time.Time, categories []core.FinancialCategory) (ImportResult, error) {
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open Excel: %w", err)
	}
	defer file.Close()
	sheet := ""
	for _, name := range file.GetSheetList() {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "ддс") {
			sheet = name
			break
		}
		if sheet == "" && strings.Contains(lower, "p&l") {
			sheet = name
		}
	}
	if sheet == "" {
		return ImportResult{}, fmt.Errorf("не найден лист ДДС или P&L")
	}
	rows, err := file.GetRows(sheet)
	if err != nil {
		return ImportResult{}, err
	}
	headerRow := findDayHeader(rows)
	if headerRow < 0 {
		return ImportResult{}, fmt.Errorf("не найдена строка с днями месяца")
	}
	categoryIDs := map[string]uint{}
	for _, category := range categories {
		categoryIDs[category.Code] = category.ID
	}
	result := ImportResult{}
	section := ""
	for rowIndex := headerRow + 1; rowIndex < len(rows); rowIndex++ {
		row := rows[rowIndex]
		label := normalizeLabel(cell(row, 0))
		switch {
		case label == "выручка":
			section = "revenue"
			continue
		case strings.Contains(label, "food coast") || strings.Contains(label, "food cost"):
			section = "cogs"
			continue
		case strings.Contains(label, "валовая прибыль"):
			section = ""
			continue
		case strings.Contains(label, "прямые контролируемые"):
			section = "expense"
			continue
		}
		code := templateCategoryCode(section, label)
		categoryID, ok := categoryIDs[code]
		if !ok || code == "" {
			continue
		}
		for column := 1; column < len(row); column++ {
			day, err := strconv.Atoi(strings.TrimSpace(cell(rows[headerRow], column)))
			if err != nil || day < 1 || day > 31 {
				continue
			}
			amount, err := parseAmount(cell(row, column))
			if err != nil || amount.IsZero() {
				continue
			}
			date := time.Date(month.Year(), month.Month(), day, 0, 0, 0, 0, month.Location())
			if date.Month() != month.Month() {
				continue
			}
			direction := "expense"
			if section == "revenue" {
				direction = "income"
			}
			id := categoryID
			result.Entries = append(result.Entries, core.FinancialEntry{
				RestaurantID: restaurantID, CategoryID: &id, OccurredAt: date,
				Amount: amount.Abs(), Direction: direction, Description: cell(row, 0),
				Source: "excel", ExternalID: fmt.Sprintf("%s:%d:%d:%s", sheet, rowIndex+1, column+1, amount.String()),
			})
		}
	}
	if len(result.Entries) == 0 {
		result.Errors = append(result.Errors, "в распознанных строках не найдено числовых операций")
	}
	return result, nil
}

func ReadPreview(reader io.Reader, maxRows int) (Preview, error) {
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return Preview{}, fmt.Errorf("open Excel: %w", err)
	}
	defer file.Close()
	result := Preview{}
	for _, name := range file.GetSheetList() {
		iterator, err := file.Rows(name)
		if err != nil {
			return Preview{}, err
		}
		rows := make([][]string, 0, maxRows)
		for iterator.Next() && len(rows) < maxRows {
			row, err := iterator.Columns()
			if err != nil {
				_ = iterator.Close()
				return Preview{}, err
			}
			if row == nil {
				row = []string{}
			}
			rows = append(rows, row)
		}
		_ = iterator.Close()
		result.Sheets = append(result.Sheets, SheetPreview{Name: name, Rows: rows})
	}
	return result, nil
}

func Import(reader io.Reader, restaurantID uint, mapping ColumnMapping, categories []core.FinancialCategory) (ImportResult, error) {
	file, err := excelize.OpenReader(reader)
	if err != nil {
		return ImportResult{}, fmt.Errorf("open Excel: %w", err)
	}
	defer file.Close()

	rows, err := file.GetRows(mapping.Sheet)
	if err != nil {
		return ImportResult{}, err
	}

	categoryByCode := map[string]uint{}
	for _, category := range categories {
		categoryByCode[strings.ToLower(strings.TrimSpace(category.Code))] = category.ID
	}

	start := 0
	if mapping.HasHeader {
		start = 1
	}

	result := ImportResult{}
	for index := start; index < len(rows); index++ {
		row := rows[index]
		if blankRow(row) {
			continue
		}

		date, err := parseDate(cell(row, mapping.Date))
		if err != nil {
			// Тихо пропускаем строки без правильной даты (например, шапки и текст)
			continue
		}

		amount, err := parseAmount(cell(row, mapping.Amount))
		if err != nil || amount.IsZero() {
			// Тихо пропускаем строки без суммы
			continue
		}

		direction := strings.ToLower(strings.TrimSpace(cell(row, mapping.Direction)))
		switch direction {
		case "приход", "доход", "income", "in":
			direction = "income"
		case "расход", "expense", "out":
			direction = "expense"
		default:
			// Умное определение: если колонка направления пустая или ее нет, смотрим на знак суммы
			if amount.IsNegative() {
				direction = "expense"
			} else {
				direction = "income"
			}
		}

		entry := core.FinancialEntry{
			RestaurantID:  restaurantID,
			OccurredAt:    date,
			Amount:        amount.Abs(), // В базу всегда сохраняем положительное число
			Direction:     direction,
			Description:   cell(row, mapping.Description),
			Counterparty:  cell(row, mapping.Counterparty),
			PaymentMethod: cell(row, mapping.PaymentMethod),
			Source:        "excel",
			ExternalID:    fmt.Sprintf("%s:%d:%s", mapping.Sheet, index+1, amount.String()),
		}

		if categoryID, ok := categoryByCode[strings.ToLower(strings.TrimSpace(cell(row, mapping.CategoryCode)))]; ok {
			entry.CategoryID = &categoryID
		}

		result.Entries = append(result.Entries, entry)
	}
	return result, nil
}

func ExportReports(writer io.Writer, pnl core.PnLReport, cashFlow core.CashFlowReport, payroll core.PayrollReport) error {
	file := excelize.NewFile()
	defer file.Close()
	defaultSheet := file.GetSheetName(0)
	_ = file.SetSheetName(defaultSheet, "P&L")
	if err := writePnL(file, "P&L", pnl); err != nil {
		return err
	}
	if _, err := file.NewSheet("ДДС"); err != nil {
		return err
	}
	if err := writeCashFlow(file, "ДДС", cashFlow); err != nil {
		return err
	}
	if _, err := file.NewSheet("Зарплата"); err != nil {
		return err
	}
	if err := writePayroll(file, "Зарплата", payroll); err != nil {
		return err
	}
	return file.Write(writer)
}

func ExportPayroll(writer io.Writer, payroll core.PayrollReport) error {
	file := excelize.NewFile()
	defer file.Close()
	sheet := file.GetSheetName(0)
	_ = file.SetSheetName(sheet, "Зарплата")
	_ = file.SetCellValue("Зарплата", "A1", "Расчёт зарплаты")
	_ = file.SetCellValue("Зарплата", "A2", payroll.From.Format("02.01.2006")+" — "+payroll.To.Format("02.01.2006"))
	headers := []interface{}{"Сотрудник", "Должность", "Часы", "Начислено", "KPI", "Бонус", "Аванс", "Удержание", "К выплате"}
	if err := file.SetSheetRow("Зарплата", "A4", &headers); err != nil {
		return err
	}
	row := 5
	for _, line := range payroll.Lines {
		values := []interface{}{line.Name, line.Position, line.Hours.InexactFloat64(), line.Gross.InexactFloat64(), line.KPI.InexactFloat64(), line.Bonus.InexactFloat64(), line.Advance.InexactFloat64(), line.Deduction.InexactFloat64(), line.Net.InexactFloat64()}
		cell, _ := excelize.CoordinatesToCellName(1, row)
		if err := file.SetSheetRow("Зарплата", cell, &values); err != nil {
			return err
		}
		row++
	}
	_ = file.SetCellValue("Зарплата", fmt.Sprintf("A%d", row+1), "Итого")
	_ = file.SetCellValue("Зарплата", fmt.Sprintf("I%d", row+1), payroll.Total.InexactFloat64())
	return file.Write(writer)
}

func writePnL(file *excelize.File, sheet string, report core.PnLReport) error {
	headers := []interface{}{"Статья", "Факт", "План", "Отклонение", "% от выручки"}
	if err := file.SetSheetRow(sheet, "A3", &headers); err != nil {
		return err
	}
	_ = file.SetCellValue(sheet, "A1", "P&L — отчёт о прибыли и убытках")
	_ = file.SetCellValue(sheet, "A2", report.From.Format("02.01.2006")+" — "+report.To.Format("02.01.2006"))
	row := 4
	for _, line := range report.Lines {
		values := []interface{}{line.Name, line.Actual.InexactFloat64(), line.Plan.InexactFloat64(), line.Variance.InexactFloat64(), line.Percent.InexactFloat64()}
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = file.SetSheetRow(sheet, cell, &values)
		row++
	}
	summaries := [][]interface{}{
		{"Выручка", report.Revenue.InexactFloat64()},
		{"Себестоимость", report.COGS.InexactFloat64()},
		{"Валовая прибыль", report.GrossProfit.InexactFloat64()},
		{"Контролируемая прибыль", report.ControlledProfit.InexactFloat64()},
		{"EBITDA", report.EBITDA.InexactFloat64()},
		{"Операционная прибыль", report.OperatingProfit.InexactFloat64()},
	}
	row++
	for _, values := range summaries {
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = file.SetSheetRow(sheet, cell, &values)
		row++
	}
	return styleReport(file, sheet, row, 5)
}

func writeCashFlow(file *excelize.File, sheet string, report core.CashFlowReport) error {
	_ = file.SetCellValue(sheet, "A1", "Движение денежных средств")
	headers := []interface{}{"Статья", "Сумма"}
	_ = file.SetSheetRow(sheet, "A3", &headers)
	row := 4
	for _, line := range report.Lines {
		values := []interface{}{line.Name, line.Actual.InexactFloat64()}
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = file.SetSheetRow(sheet, cell, &values)
		row++
	}
	for _, values := range [][]interface{}{
		{"Поступления", report.Inflows.InexactFloat64()},
		{"Выплаты", report.Outflows.InexactFloat64()},
		{"Чистый денежный поток", report.NetCashFlow.InexactFloat64()},
	} {
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = file.SetSheetRow(sheet, cell, &values)
		row++
	}
	return styleReport(file, sheet, row, 2)
}

//сотрудники, должность, часы, начислено, kpi, бонус, аванс, удержание, к выплате

func writePayroll(file *excelize.File, sheet string, report core.PayrollReport) error {
	_ = file.SetCellValue(sheet, "A1", "Расчёт заработной платы")
	headers := []interface{}{"Сотрудник", "Должность", "Часы", "Начислено", "KPI", "Бонус", "Аванс", "Удержание", "К выплате"}
	_ = file.SetSheetRow(sheet, "A3", &headers)
	row := 4
	for _, line := range report.Lines {
		values := []interface{}{line.Name, line.Position, line.Hours.InexactFloat64(), line.Gross.InexactFloat64(), line.KPI.InexactFloat64(), line.Bonus.InexactFloat64(), line.Advance.InexactFloat64(), line.Deduction.InexactFloat64(), line.Net.InexactFloat64()}
		cell, _ := excelize.CoordinatesToCellName(1, row)
		_ = file.SetSheetRow(sheet, cell, &values)
		row++
	}
	return styleReport(file, sheet, row, 9)
}

func styleReport(file *excelize.File, sheet string, lastRow, columns int) error {
	header, _ := file.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"2F5597"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	money, _ := file.NewStyle(&excelize.Style{NumFmt: 4})
	percent, _ := file.NewStyle(&excelize.Style{NumFmt: 10})
	lastColumn, _ := excelize.ColumnNumberToName(columns)
	_ = file.SetCellStyle(sheet, "A3", lastColumn+"3", header)
	if columns > 1 {
		_ = file.SetCellStyle(sheet, "B4", lastColumn+strconv.Itoa(lastRow), money)
	}
	if columns == 5 {
		_ = file.SetCellStyle(sheet, "E4", "E"+strconv.Itoa(lastRow), percent)
	}
	_ = file.SetColWidth(sheet, "A", "A", 44)
	if columns > 1 {
		_ = file.SetColWidth(sheet, "B", lastColumn, 16)
	}
	_ = file.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 3, TopLeftCell: "A4", ActivePane: "bottomLeft"})
	return nil
}

func cell(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func blankRow(row []string) bool {
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func parseAmount(value string) (decimal.Decimal, error) {
	value = strings.ReplaceAll(strings.TrimSpace(value), " ", "")
	value = strings.ReplaceAll(value, "\u00a0", "")

	// Умная обработка точек и запятых в числах (например, 184,965.50 или 184.965,50)
	if strings.Contains(value, ".") && strings.Contains(value, ",") {
		lastDot := strings.LastIndex(value, ".")
		lastComma := strings.LastIndex(value, ",")
		if lastDot > lastComma {
			// Американский формат (1,234.56) -> удаляем запятые
			value = strings.ReplaceAll(value, ",", "")
		} else {
			// Европейский формат (1.234,56) -> удаляем точки, запятую меняем на точку
			value = strings.ReplaceAll(value, ".", "")
			value = strings.ReplaceAll(value, ",", ".")
		}
	} else if strings.Contains(value, ",") {
		// Только запятые (например, 184965,50 или 1,234,567)
		if strings.Count(value, ",") > 1 {
			// Много запятых = это разделитель тысяч
			value = strings.ReplaceAll(value, ",", "")
		} else {
			// Одна запятая = десятичный разделитель
			value = strings.ReplaceAll(value, ",", ".")
		}
	}

	// Если остались только точки в качестве разделителя тысяч (1.234.567)
	if strings.Count(value, ".") > 1 {
		value = strings.ReplaceAll(value, ".", "")
	}

	return decimal.NewFromString(value)
}

func parseDate(value string) (time.Time, error) {
	value = strings.TrimSpace(value)

	// Если дата пришла со временем (2022-11-01 00:00:00) - отрезаем время
	if strings.Contains(value, " ") {
		value = strings.Split(value, " ")[0]
	}

	// Все возможные форматы дат, включая американские, европейские, с точками, слэшами и тире
	layouts := []string{
		"2006-01-02", "02.01.2006", "02/01/2006", "02-01-2006",
		"01-02-2006", "01.02.2006", "01/02/2006",
		"02.01.06", "02/01/06", "02-01-06",
		"01.02.06", "01/02/06", "01-02-06",
		"2006/01/02", "2006.01.02",
	}

	for _, layout := range layouts {
		if result, err := time.Parse(layout, value); err == nil {
			return result, nil
		}
	}

	if serial, err := strconv.ParseFloat(value, 64); err == nil {
		return excelize.ExcelDateToTime(serial, false)
	}

	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func normalizeLabel(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func templateCategoryCode(section, label string) string {
	if label == "" || isTemplateSummary(label) || strings.Contains(label, "итого") || strings.Contains(label, "прибыл") ||
		strings.Contains(label, "нал (") || strings.Contains(label, "б/н") {
		return ""
	}
	if section == "revenue" {
		switch {
		case strings.Contains(label, "кухня"):
			return "revenue_kitchen"
		case strings.Contains(label, "спирт"):
			return "revenue_alcohol"
		case strings.Contains(label, "безалког"):
			return "revenue_soft"
		case strings.Contains(label, "кальян"):
			return "revenue_hookah"
		case strings.Contains(label, "прочее"):
			return "revenue_other"
		}
	}
	if section == "cogs" {
		switch {
		case strings.Contains(label, "кух") || strings.Contains(label, "материалы для производства"):
			return "cogs_kitchen"
		case strings.Contains(label, "спирт"):
			return "cogs_alcohol"
		case strings.Contains(label, "безалког"):
			return "cogs_soft"
		case strings.Contains(label, "табак") || strings.Contains(label, "уголь"):
			return "cogs_hookah"
		}
	}
	if section == "expense" {
		switch {
		case strings.Contains(label, "оплата труда") || strings.Contains(label, "зп ") || strings.Contains(label, "фот"):
			return "payroll"
		case strings.Contains(label, "аренд"):
			return "rent"
		case strings.Contains(label, "маркет") || strings.Contains(label, "реклам") || strings.Contains(label, "смм") || strings.Contains(label, "фотограф"):
			return "marketing"
		case strings.Contains(label, "эквайр"):
			return "acquiring"
		case strings.Contains(label, "электр") || strings.Contains(label, "вода") || strings.Contains(label, "отоп") || strings.Contains(label, "интернет") || strings.Contains(label, "связ"):
			return "utilities"
		case strings.Contains(label, "налог") || strings.Contains(label, "усн"):
			return "tax"
		case strings.Contains(label, "материал") || strings.Contains(label, "упаков") || strings.Contains(label, "ремонт") || strings.Contains(label, "посуда") || strings.Contains(label, "инвентар"):
			return "materials"
		case strings.Contains(label, "ук"):
			return "management"
		default:
			return "services"
		}
	}
	return ""
}

func isTemplateSummary(label string) bool {
	switch label {
	case "оплата труда",
		"коммунальные услуги",
		"другие услуги",
		"материалы",
		"маркетинг",
		"аренда",
		"налоги",
		"неконтролируемые операционные расходы",
		"накладные расходы":
		return true
	default:
		return false
	}
}
func findDayHeader(rows [][]string) int {
	for rowIndex, row := range rows {
		matches := 0
		for column := 1; column < len(row); column++ {
			day, err := strconv.Atoi(strings.TrimSpace(row[column]))
			if err == nil && day >= 1 && day <= 31 {
				matches++
			}
		}
		if matches >= 7 {
			return rowIndex
		}
	}
	return -1
}
