package core

import (
	"time"

	"github.com/shopspring/decimal"
)

type CategoryKind string

const (
	CategoryRevenue      CategoryKind = "revenue"
	CategoryCOGS         CategoryKind = "cogs"
	CategoryControlled   CategoryKind = "controlled_expense"
	CategoryPayroll      CategoryKind = "payroll"
	CategoryUncontrolled CategoryKind = "uncontrolled_expense"
	CategoryOverhead     CategoryKind = "overhead"
	CategoryDepreciation CategoryKind = "depreciation"
	CategoryTax          CategoryKind = "tax"
	CategoryCashIn       CategoryKind = "cash_in"
	CategoryCashOut      CategoryKind = "cash_out"
)

type Restaurant struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:200;not null;uniqueIndex" json:"name"`
	Currency  string    `gorm:"size:3;not null;default:RUB" json:"currency"`
	Timezone  string    `gorm:"size:80;not null;default:Europe/Moscow" json:"timezone"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type FinancialCategory struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	RestaurantID uint         `gorm:"not null;uniqueIndex:ux_category_code" json:"restaurant_id"`
	Restaurant   Restaurant   `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	ParentID     *uint        `json:"parent_id,omitempty"`
	Code         string       `gorm:"size:100;not null;uniqueIndex:ux_category_code" json:"code"`
	Name         string       `gorm:"size:255;not null" json:"name"`
	Kind         CategoryKind `gorm:"size:40;not null;index" json:"kind"`
	Report       string       `gorm:"size:10;not null;default:BOTH" json:"report"`
	SortOrder    int          `gorm:"not null;default:0" json:"sort_order"`
	Active       bool         `gorm:"not null;default:true" json:"active"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type FinancialEntry struct {
	ID            uint               `gorm:"primaryKey" json:"id"`
	RestaurantID  uint               `gorm:"not null;index:idx_entry_period" json:"restaurant_id"`
	Restaurant    Restaurant         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	CategoryID    *uint              `gorm:"index" json:"category_id,omitempty"`
	Category      *FinancialCategory `gorm:"constraint:OnUpdate:CASCADE,OnDelete:SET NULL" json:"-"`
	OccurredAt    time.Time          `gorm:"not null;index:idx_entry_period" json:"occurred_at"`
	Amount        decimal.Decimal    `gorm:"type:decimal(18,2);not null" json:"amount"`
	Direction     string             `gorm:"size:10;not null;index;check:direction IN ('income','expense')" json:"direction"`
	PaymentMethod string             `gorm:"size:30" json:"payment_method"`
	Description   string             `gorm:"size:500" json:"description"`
	Counterparty  string             `gorm:"size:255" json:"counterparty"`
	Tags          string             `gorm:"type:text" json:"tags,omitempty"`
	Source        string             `gorm:"size:30;not null;default:manual;index" json:"source"`
	ExternalID    string             `gorm:"size:255;index" json:"external_id,omitempty"`
	Metadata      string             `gorm:"type:text" json:"metadata,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

type PlanValue struct {
	ID           uint              `gorm:"primaryKey" json:"id"`
	RestaurantID uint              `gorm:"not null;uniqueIndex:ux_plan_value" json:"restaurant_id"`
	Restaurant   Restaurant        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	CategoryID   uint              `gorm:"not null;uniqueIndex:ux_plan_value" json:"category_id"`
	Category     FinancialCategory `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Month        time.Time         `gorm:"type:date;not null;uniqueIndex:ux_plan_value" json:"month"`
	Amount       decimal.Decimal   `gorm:"type:decimal(18,2);not null" json:"amount"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

type CalculationRule struct {
	ID               uint            `gorm:"primaryKey" json:"id"`
	RestaurantID     uint            `gorm:"not null;index" json:"restaurant_id"`
	Restaurant       Restaurant      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Name             string          `gorm:"size:255;not null" json:"name"`
	RuleType         string          `gorm:"size:20;not null;index" json:"rule_type"`
	Priority         int             `gorm:"not null;default:100" json:"priority"`
	Active           bool            `gorm:"not null;default:true" json:"active"`
	MatchField       string          `gorm:"size:30" json:"match_field,omitempty"`
	MatchOperator    string          `gorm:"size:30" json:"match_operator,omitempty"`
	MatchValue       string          `gorm:"size:255" json:"match_value,omitempty"`
	SourceCategoryID *uint           `json:"source_category_id,omitempty"`
	SourceMetric     string          `gorm:"size:50" json:"source_metric,omitempty"`
	TargetCategoryID *uint           `json:"target_category_id,omitempty"`
	Operation        string          `gorm:"size:30" json:"operation,omitempty"`
	Rate             decimal.Decimal `gorm:"type:decimal(18,6);not null;default:0" json:"rate"`
	FixedAmount      decimal.Decimal `gorm:"type:decimal(18,2);not null;default:0" json:"fixed_amount"`
	StopProcessing   bool            `gorm:"not null;default:true" json:"stop_processing"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type Employee struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	RestaurantID uint            `gorm:"not null;index" json:"restaurant_id"`
	Restaurant   Restaurant      `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Name         string          `gorm:"size:255;not null" json:"name"`
	Position     string          `gorm:"size:150;not null" json:"position"`
	HourlyRate   decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"hourly_rate"`
	MonthlyRate  decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"monthly_rate"`
	KPIPercent   decimal.Decimal `gorm:"type:decimal(8,4);not null;default:0" json:"kpi_percent"`
	Active       bool            `gorm:"not null;default:true" json:"active"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type Shift struct {
	ID         uint            `gorm:"primaryKey" json:"id"`
	EmployeeID uint            `gorm:"not null;index" json:"employee_id"`
	Employee   Employee        `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Date       time.Time       `gorm:"type:date;not null;index" json:"date"`
	Hours      decimal.Decimal `gorm:"type:decimal(8,2);not null" json:"hours"`
	Bonus      decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"bonus"`
	Advance    decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"advance"`
	Deduction  decimal.Decimal `gorm:"type:decimal(12,2);not null;default:0" json:"deduction"`
	Comment    string          `gorm:"size:500" json:"comment"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

type POSConnection struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	RestaurantID uint       `gorm:"not null;index" json:"restaurant_id"`
	Restaurant   Restaurant `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE" json:"-"`
	Provider     string     `gorm:"size:30;not null" json:"provider"`
	Name         string     `gorm:"size:150;not null" json:"name"`
	BaseURL      string     `gorm:"size:500" json:"base_url"`
	APIKey       string     `gorm:"type:text" json:"-"`
	Settings     string     `gorm:"type:text" json:"settings,omitempty"`
	Active       bool       `gorm:"not null;default:true" json:"active"`
	LastSyncAt   *time.Time `json:"last_sync_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
