package core

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestBuildPnLAndCalculationRule(t *testing.T) {
	revenueID, cogsID, rentID, acquiringID := uint(1), uint(2), uint(3), uint(4)
	categories := []FinancialCategory{
		{ID: revenueID, Code: "revenue", Name: "Выручка", Kind: CategoryRevenue, Report: "BOTH", Active: true},
		{ID: cogsID, Code: "cogs", Name: "Себестоимость", Kind: CategoryCOGS, Report: "PNL", Active: true},
		{ID: rentID, Code: "rent", Name: "Аренда", Kind: CategoryUncontrolled, Report: "PNL", Active: true},
		{ID: acquiringID, Code: "acquiring", Name: "Эквайринг", Kind: CategoryControlled, Report: "PNL", Active: true},
	}
	entries := []FinancialEntry{
		{CategoryID: &revenueID, Amount: decimal.NewFromInt(1_000_000)},
		{CategoryID: &cogsID, Amount: decimal.NewFromInt(250_000)},
		{CategoryID: &rentID, Amount: decimal.NewFromInt(100_000)},
	}
	rules := []CalculationRule{{
		RuleType: "calculation", Active: true, SourceMetric: "revenue", TargetCategoryID: &acquiringID,
		Operation: "percent_of", Rate: decimal.RequireFromString("0.018"),
	}}
	report := BuildPnL(1, time.Now(), time.Now(), categories, entries, nil, rules)
	if !report.Revenue.Equal(decimal.NewFromInt(1_000_000)) {
		t.Fatalf("revenue = %s", report.Revenue)
	}
	if !report.ControlledExpenses.Equal(decimal.NewFromInt(18_000)) {
		t.Fatalf("acquiring = %s", report.ControlledExpenses)
	}
	if !report.EBITDA.Equal(decimal.NewFromInt(632_000)) {
		t.Fatalf("EBITDA = %s", report.EBITDA)
	}
}

func TestClassificationAndBreakEven(t *testing.T) {
	target := uint(9)
	entry := FinancialEntry{Description: "Оплата аренды за июль"}
	ApplyClassificationRules(&entry, []CalculationRule{{
		RuleType: "classification", Active: true, MatchField: "description",
		MatchOperator: "contains", MatchValue: "аренд", TargetCategoryID: &target,
	}})
	if entry.CategoryID == nil || *entry.CategoryID != target {
		t.Fatal("classification rule did not assign target category")
	}
	got := BreakEvenRevenue(decimal.NewFromInt(600_000), decimal.RequireFromString("0.25"))
	if !got.Equal(decimal.NewFromInt(800_000)) {
		t.Fatalf("break-even = %s", got)
	}
}

func TestRuleEngineUsesPriorityAndDoesNotMutateRules(t *testing.T) {
	firstTarget, secondTarget := uint(1), uint(2)
	rules := []CalculationRule{
		{
			ID: 2, RuleType: "classification", Active: true, Priority: 200,
			MatchField: "description", MatchOperator: "contains", MatchValue: "аренд",
			TargetCategoryID: &secondTarget, StopProcessing: true,
		},
		{
			ID: 1, RuleType: "classification", Active: true, Priority: 100,
			MatchField: "description", MatchOperator: "contains", MatchValue: "аренд",
			TargetCategoryID: &firstTarget, StopProcessing: true,
		},
	}
	entry := FinancialEntry{Description: "аренда"}
	NewRuleEngine(rules).Classify(&entry)

	if entry.CategoryID == nil || *entry.CategoryID != firstTarget {
		t.Fatalf("priority was ignored: %+v", entry.CategoryID)
	}
	if rules[0].ID != 2 {
		t.Fatal("rule engine mutated caller-owned slice")
	}
}

func TestAmountClassificationRule(t *testing.T) {
	target := uint(4)
	entry := FinancialEntry{Amount: decimal.NewFromInt(15_000)}
	NewRuleEngine([]CalculationRule{{
		RuleType: "classification", Active: true, MatchField: "amount",
		MatchOperator: "greater_than", MatchValue: "10000", TargetCategoryID: &target,
	}}).Classify(&entry)
	if entry.CategoryID == nil || *entry.CategoryID != target {
		t.Fatal("numeric amount filter was not applied")
	}
}

func BenchmarkRuleEngineClassification(b *testing.B) {
	target := uint(1)
	rules := make([]CalculationRule, 100)
	for index := range rules {
		rules[index] = CalculationRule{
			ID: uint(index + 1), RuleType: "classification", Active: true, Priority: index,
			MatchField: "description", MatchOperator: "contains", MatchValue: "match",
			TargetCategoryID: &target,
		}
	}
	engine := NewRuleEngine(rules)
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		entry := FinancialEntry{Description: "no value"}
		engine.Classify(&entry)
	}
}
