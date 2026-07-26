package core

import (
	"time"

	"github.com/shopspring/decimal"
)

type PayrollLine struct {
	EmployeeID uint            `json:"employee_id"`
	Name       string          `json:"name"`
	Position   string          `json:"position"`
	ShiftCount int             `json:"shift_count"`
	Hours      decimal.Decimal `json:"hours"`
	Gross      decimal.Decimal `json:"gross"`
	KPI        decimal.Decimal `json:"kpi"`
	Bonus      decimal.Decimal `json:"bonus"`
	Advance    decimal.Decimal `json:"advance"`
	Deduction  decimal.Decimal `json:"deduction"`
	Net        decimal.Decimal `json:"net"`
}

type PayrollReport struct {
	From  time.Time       `json:"from"`
	To    time.Time       `json:"to"`
	Total decimal.Decimal `json:"total"`
	Lines []PayrollLine   `json:"lines"`
}

func BuildPayroll(from, to time.Time, employees []Employee, shifts []Shift, revenue decimal.Decimal) PayrollReport {
	byEmployee := map[uint][]Shift{}
	for _, shift := range shifts {
		byEmployee[shift.EmployeeID] = append(byEmployee[shift.EmployeeID], shift)
	}
	report := PayrollReport{From: from, To: to}
	for _, employee := range employees {
		line := PayrollLine{EmployeeID: employee.ID, Name: employee.Name, Position: employee.Position}
		for _, shift := range byEmployee[employee.ID] {
			line.ShiftCount++
			line.Hours = line.Hours.Add(shift.Hours)
			line.Bonus = line.Bonus.Add(shift.Bonus)
			line.Advance = line.Advance.Add(shift.Advance)
			line.Deduction = line.Deduction.Add(shift.Deduction)
		}
		line.Gross = employee.MonthlyRate
		if line.Gross.IsZero() {
			line.Gross = line.Hours.Mul(employee.HourlyRate)
		}
		line.KPI = revenue.Mul(employee.KPIPercent)
		line.Net = line.Gross.Add(line.KPI).Add(line.Bonus).Sub(line.Advance).Sub(line.Deduction)
		report.Total = report.Total.Add(line.Net)
		report.Lines = append(report.Lines, line)
	}
	return report
}
