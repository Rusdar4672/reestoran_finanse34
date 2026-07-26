package core

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

type ReportLine struct {
	CategoryID uint            `json:"category_id"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Kind       CategoryKind    `json:"kind"`
	Actual     decimal.Decimal `json:"actual"`
	Plan       decimal.Decimal `json:"plan"`
	Variance   decimal.Decimal `json:"variance"`
	Percent    decimal.Decimal `json:"percent"`
	Calculated bool            `json:"calculated"`
}

type PnLReport struct {
	RestaurantID       uint            `json:"restaurant_id"`
	From               time.Time       `json:"from"`
	To                 time.Time       `json:"to"`
	Revenue            decimal.Decimal `json:"revenue"`
	COGS               decimal.Decimal `json:"cogs"`
	GrossProfit        decimal.Decimal `json:"gross_profit"`
	ControlledExpenses decimal.Decimal `json:"controlled_expenses"`
	Payroll            decimal.Decimal `json:"payroll"`
	ControlledProfit   decimal.Decimal `json:"controlled_profit"`
	Uncontrolled       decimal.Decimal `json:"uncontrolled_expenses"`
	Overhead           decimal.Decimal `json:"overhead"`
	EBITDA             decimal.Decimal `json:"ebitda"`
	Depreciation       decimal.Decimal `json:"depreciation"`
	Tax                decimal.Decimal `json:"tax"`
	OperatingProfit    decimal.Decimal `json:"operating_profit"`
	Margin             decimal.Decimal `json:"margin"`
	Lines              []ReportLine    `json:"lines"`
}

type CashFlowReport struct {
	RestaurantID uint            `json:"restaurant_id"`
	From         time.Time       `json:"from"`
	To           time.Time       `json:"to"`
	Inflows      decimal.Decimal `json:"inflows"`
	Outflows     decimal.Decimal `json:"outflows"`
	NetCashFlow  decimal.Decimal `json:"net_cash_flow"`
	Lines        []ReportLine    `json:"lines"`
}

func BuildPnL(restaurantID uint, from, to time.Time, categories []FinancialCategory, entries []FinancialEntry, plans []PlanValue, rules []CalculationRule) PnLReport {
	return BuildPnLFromTotals(restaurantID, from, to, categories, aggregateEntries(entries), plans, rules)
}

func BuildPnLFromTotals(restaurantID uint, from, to time.Time, categories []FinancialCategory, amounts map[uint]decimal.Decimal, plans []PlanValue, rules []CalculationRule) PnLReport {
	plansByCategory := map[uint]decimal.Decimal{}
	for _, plan := range plans {
		plansByCategory[plan.CategoryID] = plansByCategory[plan.CategoryID].Add(plan.Amount)
	}
	amounts = cloneAmounts(amounts)
	calculated := NewRuleEngine(rules).ApplyCalculations(categories, amounts)
	lines := make([]ReportLine, 0, len(categories))
	report := PnLReport{RestaurantID: restaurantID, From: from, To: to}

	categories = slices.Clone(categories)
	sort.Slice(categories, func(i, j int) bool { return categories[i].SortOrder < categories[j].SortOrder })
	for _, category := range categories {
		if !category.Active || (category.Report != "PNL" && category.Report != "BOTH") {
			continue
		}
		actual := amounts[category.ID]
		plan := plansByCategory[category.ID]
		line := ReportLine{
			CategoryID: category.ID, Code: category.Code, Name: category.Name, Kind: category.Kind,
			Actual: actual, Plan: plan, Variance: actual.Sub(plan), Calculated: calculated[category.ID],
		}
		lines = append(lines, line)
		switch category.Kind {
		case CategoryRevenue:
			report.Revenue = report.Revenue.Add(actual)
		case CategoryCOGS:
			report.COGS = report.COGS.Add(actual)
		case CategoryControlled:
			report.ControlledExpenses = report.ControlledExpenses.Add(actual)
		case CategoryPayroll:
			report.Payroll = report.Payroll.Add(actual)
		case CategoryUncontrolled:
			report.Uncontrolled = report.Uncontrolled.Add(actual)
		case CategoryOverhead:
			report.Overhead = report.Overhead.Add(actual)
		case CategoryDepreciation:
			report.Depreciation = report.Depreciation.Add(actual)
		case CategoryTax:
			report.Tax = report.Tax.Add(actual)
		}
	}
	for i := range lines {
		if !report.Revenue.IsZero() {
			lines[i].Percent = lines[i].Actual.Div(report.Revenue)
		}
	}
	report.GrossProfit = report.Revenue.Sub(report.COGS)
	report.ControlledProfit = report.GrossProfit.Sub(report.ControlledExpenses).Sub(report.Payroll)
	report.EBITDA = report.ControlledProfit.Sub(report.Uncontrolled).Sub(report.Overhead)
	report.OperatingProfit = report.EBITDA.Sub(report.Depreciation).Sub(report.Tax)
	if !report.Revenue.IsZero() {
		report.Margin = report.OperatingProfit.Div(report.Revenue)
	}
	report.Lines = lines
	return report
}

func BuildCashFlow(restaurantID uint, from, to time.Time, categories []FinancialCategory, entries []FinancialEntry) CashFlowReport {
	return BuildCashFlowFromTotals(restaurantID, from, to, categories, aggregateEntries(entries))
}

func BuildCashFlowFromTotals(restaurantID uint, from, to time.Time, categories []FinancialCategory, amounts map[uint]decimal.Decimal) CashFlowReport {
	report := CashFlowReport{RestaurantID: restaurantID, From: from, To: to}
	for _, category := range categories {
		if !category.Active || (category.Report != "DDS" && category.Report != "BOTH") {
			continue
		}
		amount := amounts[category.ID]
		report.Lines = append(report.Lines, ReportLine{
			CategoryID: category.ID, Code: category.Code, Name: category.Name, Kind: category.Kind, Actual: amount,
		})
		if category.Kind == CategoryCashIn || category.Kind == CategoryRevenue {
			report.Inflows = report.Inflows.Add(amount)
		} else {
			report.Outflows = report.Outflows.Add(amount)
		}
	}
	report.NetCashFlow = report.Inflows.Sub(report.Outflows)
	return report
}

func aggregateEntries(entries []FinancialEntry) map[uint]decimal.Decimal {
	result := map[uint]decimal.Decimal{}
	for _, entry := range entries {
		if entry.CategoryID == nil {
			continue
		}
		result[*entry.CategoryID] = result[*entry.CategoryID].Add(entry.Amount.Abs())
	}
	return result
}

func MatchRule(entry FinancialEntry, rule CalculationRule) bool {
	if rule.MatchField == "amount" {
		expected, err := decimal.NewFromString(strings.TrimSpace(rule.MatchValue))
		if err != nil {
			return false
		}
		switch rule.MatchOperator {
		case "equals":
			return entry.Amount.Equal(expected)
		case "greater_than":
			return entry.Amount.GreaterThan(expected)
		case "greater_or_equal":
			return entry.Amount.GreaterThanOrEqual(expected)
		case "less_than":
			return entry.Amount.LessThan(expected)
		case "less_or_equal":
			return entry.Amount.LessThanOrEqual(expected)
		default:
			return false
		}
	}
	var value string
	switch rule.MatchField {
	case "description":
		value = entry.Description
	case "counterparty":
		value = entry.Counterparty
	case "payment_method":
		value = entry.PaymentMethod
	case "source":
		value = entry.Source
	case "direction":
		value = entry.Direction
	default:
		return false
	}
	value = strings.ToLower(strings.TrimSpace(value))
	expected := strings.ToLower(strings.TrimSpace(rule.MatchValue))
	switch rule.MatchOperator {
	case "equals":
		return value == expected
	case "contains":
		return strings.Contains(value, expected)
	case "starts_with":
		return strings.HasPrefix(value, expected)
	case "ends_with":
		return strings.HasSuffix(value, expected)
	case "not_contains":
		return !strings.Contains(value, expected)
	default:
		return false
	}
}

func ApplyClassificationRules(entry *FinancialEntry, rules []CalculationRule) {
	NewRuleEngine(rules).Classify(entry)
}

func BreakEvenRevenue(fixedCosts, variableCostRatio decimal.Decimal) decimal.Decimal {
	contributionMargin := decimal.NewFromInt(1).Sub(variableCostRatio)
	if contributionMargin.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero
	}
	return fixedCosts.Div(contributionMargin)
}

type RuleEngine struct {
	classification []CalculationRule
	calculation    []CalculationRule
}

func NewRuleEngine(rules []CalculationRule) RuleEngine {
	engine := RuleEngine{}
	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		switch rule.RuleType {
		case "classification":
			if rule.TargetCategoryID != nil {
				engine.classification = append(engine.classification, rule)
			}
		case "calculation":
			if rule.TargetCategoryID != nil {
				engine.calculation = append(engine.calculation, rule)
			}
		}
	}
	byPriority := func(a, b CalculationRule) int {
		if a.Priority != b.Priority {
			return a.Priority - b.Priority
		}
		return int(a.ID) - int(b.ID)
	}
	slices.SortFunc(engine.classification, byPriority)
	slices.SortFunc(engine.calculation, byPriority)
	return engine
}

func (e RuleEngine) Classify(entry *FinancialEntry) {
	for _, rule := range e.classification {
		if !MatchRule(*entry, rule) {
			continue
		}
		entry.CategoryID = rule.TargetCategoryID
		if rule.StopProcessing {
			return
		}
	}
}

func (e RuleEngine) ApplyCalculations(categories []FinancialCategory, amounts map[uint]decimal.Decimal) map[uint]bool {
	calculated := make(map[uint]bool, len(e.calculation))
	categoryByID := make(map[uint]FinancialCategory, len(categories))
	metrics := make(map[string]decimal.Decimal, 5)
	for _, category := range categories {
		categoryByID[category.ID] = category
		metric := metricName(category.Kind)
		if metric != "" {
			metrics[metric] = metrics[metric].Add(amounts[category.ID])
		}
	}
	for _, rule := range e.calculation {
		base := decimal.Zero
		if rule.SourceCategoryID != nil {
			base = amounts[*rule.SourceCategoryID]
		} else {
			base = metrics[strings.ToLower(rule.SourceMetric)]
		}
		value := calculateRuleValue(rule, base)
		target := *rule.TargetCategoryID
		old := amounts[target]
		amounts[target] = value
		calculated[target] = true

		if category, ok := categoryByID[target]; ok {
			if metric := metricName(category.Kind); metric != "" {
				metrics[metric] = metrics[metric].Sub(old).Add(value)
			}
		}
	}
	return calculated
}

func calculateRuleValue(rule CalculationRule, base decimal.Decimal) decimal.Decimal {
	switch rule.Operation {
	case "percent_of":
		return base.Mul(rule.Rate).Add(rule.FixedAmount)
	case "copy":
		return base.Add(rule.FixedAmount)
	default:
		return rule.FixedAmount
	}
}

func metricName(kind CategoryKind) string {
	switch kind {
	case CategoryRevenue:
		return "revenue"
	case CategoryCOGS:
		return "cogs"
	case CategoryPayroll:
		return "payroll"
	case CategoryControlled:
		return "controlled_expenses"
	case CategoryUncontrolled:
		return "uncontrolled_expenses"
	default:
		return ""
	}
}

func cloneAmounts(source map[uint]decimal.Decimal) map[uint]decimal.Decimal {
	result := make(map[uint]decimal.Decimal, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
