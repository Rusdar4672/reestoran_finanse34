package core

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildPayroll(t *testing.T) {
	employees := []Employee{{
		ID: 1, Name: "Официант", HourlyRate: decimal.NewFromInt(250),
		KPIPercent: decimal.RequireFromString("0.001"),
	}}
	shifts := []Shift{{
		EmployeeID: 1, Hours: decimal.NewFromInt(100), Bonus: decimal.NewFromInt(1000),
		Advance: decimal.NewFromInt(5000), Deduction: decimal.NewFromInt(500),
	}}
	report := BuildPayroll(time.Now(), time.Now(), employees, shifts, decimal.NewFromInt(1_000_000))
	if report.Lines[0].ShiftCount != 1 {
		t.Fatalf("expected one shift, got %d", report.Lines[0].ShiftCount)
	}
	if len(report.Lines) != 1 || !report.Lines[0].Net.Equal(decimal.NewFromInt(21_500)) {
		t.Fatalf("unexpected payroll: %+v", report)
	}
}
