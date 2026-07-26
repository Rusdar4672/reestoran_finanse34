package excel

import (
	"bytes"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/xuri/excelize/v2"
	"github.com/yourusername/restaurant-finance/internal/core"
)

func TestExportReports(t *testing.T) {
	var output bytes.Buffer
	err := ExportReports(&output,
		core.PnLReport{From: time.Now(), To: time.Now(), Revenue: decimal.NewFromInt(100)},
		core.CashFlowReport{Inflows: decimal.NewFromInt(100), NetCashFlow: decimal.NewFromInt(100)},
		core.PayrollReport{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if output.Len() < 1000 {
		t.Fatalf("xlsx output is unexpectedly small: %d", output.Len())
	}
	preview, err := ReadPreview(bytes.NewReader(output.Bytes()), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Sheets) != 3 {
		t.Fatalf("sheets = %d", len(preview.Sheets))
	}
}

func TestExportPayroll(t *testing.T) {
	var output bytes.Buffer
	report := core.PayrollReport{
		From: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		To:   time.Date(2026, 7, 31, 0, 0, 0, 0, time.UTC),
		Lines: []core.PayrollLine{{
			Name: "Тест", Position: "Официант", Hours: decimal.NewFromInt(8),
			Gross: decimal.NewFromInt(3200), Net: decimal.NewFromInt(3200),
		}},
		Total: decimal.NewFromInt(3200),
	}
	if err := ExportPayroll(&output, report); err != nil {
		t.Fatal(err)
	}
	if output.Len() < 1000 {
		t.Fatalf("payroll xlsx output is unexpectedly small: %d", output.Len())
	}
}

func TestReadPreviewNormalizesEmptyRows(t *testing.T) {
	file := excelize.NewFile()
	sheet := file.GetSheetName(0)
	_ = file.SetCellValue(sheet, "A1", "Заголовок")
	_ = file.SetCellValue(sheet, "A3", "Данные")
	var data bytes.Buffer
	if err := file.Write(&data); err != nil {
		t.Fatal(err)
	}

	preview, err := ReadPreview(bytes.NewReader(data.Bytes()), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Sheets) != 1 || len(preview.Sheets[0].Rows) != 3 {
		t.Fatalf("unexpected preview: %#v", preview)
	}
	if preview.Sheets[0].Rows[1] == nil {
		t.Fatal("empty Excel row must be encoded as an empty array, not null")
	}
}

func TestImportRestaurantTemplate(t *testing.T) {
	file := excelize.NewFile()
	_ = file.SetSheetName(file.GetSheetName(0), "ДДС_июль")
	headers := []interface{}{"Статья", 1, 2, 3, 4, 5, 6, 7}
	_ = file.SetSheetRow("ДДС_июль", "A3", &headers)
	_ = file.SetCellValue("ДДС_июль", "A4", "Выручка")
	revenue := []interface{}{"Кухня", 1000, 1200}
	_ = file.SetSheetRow("ДДС_июль", "A5", &revenue)
	_ = file.SetCellValue("ДДС_июль", "A6", "Food Cost")
	cogs := []interface{}{"Материалы для производства (кухня)", 250, 300}
	_ = file.SetSheetRow("ДДС_июль", "A7", &cogs)
	_ = file.SetCellValue("ДДС_июль", "A8", "Прямые контролируемые расходы")
	rent := []interface{}{"Аренда прямая", 500}
	_ = file.SetSheetRow("ДДС_июль", "A9", &rent)
	var data bytes.Buffer
	if err := file.Write(&data); err != nil {
		t.Fatal(err)
	}
	categories := []core.FinancialCategory{
		{ID: 1, Code: "revenue_kitchen"},
		{ID: 2, Code: "cogs_kitchen"},
		{ID: 3, Code: "rent"},
	}
	result, err := ImportRestaurantTemplate(bytes.NewReader(data.Bytes()), 1, time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC), categories)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 5 {
		t.Fatalf("entries = %d, errors = %v", len(result.Entries), result.Errors)
	}
}
