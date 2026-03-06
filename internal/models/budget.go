package models

import "time"

// Budget line-item type constants.
const (
	BudgetTypeIncome  = "income"
	BudgetTypeExpense = "expense"
)

// Expense category constants.
const (
	ExpenseCatFoodMeal  = "food_meal"
	ExpenseCatFoodSnack = "food_snack"
	ExpenseCatVenueFees = "venue_fees"
	ExpenseCatMaterials = "materials"
	ExpenseCatOther     = "expense_other"
)

// Income category constants.
const (
	IncomeCatRegistrationFees = "registration_fees"
	IncomeCatTicketingFee     = "ticketing_fee"
	IncomeCatSelfPay          = "self_pay"
	IncomeCatUmedwari         = "umedwari"
	IncomeCatOther            = "income_other"
)

// BudgetItem represents a single income or expense line item.
type BudgetItem struct {
	ID          string
	EventID     string
	Type        string // "income" or "expense"
	Category    string
	Description string
	Quantity    int
	UnitAmount  int // cents
	SortOrder   int
	CreatedAt   time.Time
}

// Total returns quantity * unit_amount (in cents).
func (b *BudgetItem) Total() int {
	return b.Quantity * b.UnitAmount
}

// UnitAmountDollars returns the unit amount as dollars.
func (b *BudgetItem) UnitAmountDollars() float64 {
	return float64(b.UnitAmount) / 100.0
}

// TotalDollars returns the total as dollars.
func (b *BudgetItem) TotalDollars() float64 {
	return float64(b.Total()) / 100.0
}

// CategoryLabel returns a human-readable label for the category.
func (b *BudgetItem) CategoryLabel() string {
	return CategoryLabel(b.Category)
}

// CategoryLabel returns a human-readable label for a category key.
func CategoryLabel(cat string) string {
	switch cat {
	case ExpenseCatFoodMeal:
		return "Food (Meal)"
	case ExpenseCatFoodSnack:
		return "Food (Snack)"
	case ExpenseCatVenueFees:
		return "Venue Fees"
	case ExpenseCatMaterials:
		return "Materials"
	case ExpenseCatOther:
		return "Other Expense"
	case IncomeCatRegistrationFees:
		return "Registration Fees"
	case IncomeCatTicketingFee:
		return "Ticketing Fee"
	case IncomeCatSelfPay:
		return "Self-Pay"
	case IncomeCatUmedwari:
		return "Umedwari"
	case IncomeCatOther:
		return "Other Income"
	default:
		return cat
	}
}

// IncomeCategories returns all income category options.
func IncomeCategories() []CategoryOption {
	return []CategoryOption{
		{Key: IncomeCatRegistrationFees, Label: "Registration Fees"},
		{Key: IncomeCatTicketingFee, Label: "Ticketing Fee"},
		{Key: IncomeCatSelfPay, Label: "Self-Pay"},
		{Key: IncomeCatUmedwari, Label: "Umedwari"},
		{Key: IncomeCatOther, Label: "Other"},
	}
}

// ExpenseCategories returns all expense category options.
func ExpenseCategories() []CategoryOption {
	return []CategoryOption{
		{Key: ExpenseCatFoodMeal, Label: "Food (Meal)"},
		{Key: ExpenseCatFoodSnack, Label: "Food (Snack)"},
		{Key: ExpenseCatVenueFees, Label: "Venue Fees"},
		{Key: ExpenseCatMaterials, Label: "Materials"},
		{Key: ExpenseCatOther, Label: "Other"},
	}
}

// CategoryOption is a key-label pair for dropdown options.
type CategoryOption struct {
	Key   string
	Label string
}

// BudgetSummary holds pre-computed totals for the budget card.
type BudgetSummary struct {
	IncomeItems  []BudgetItem
	ExpenseItems []BudgetItem
	TotalIncome  int // cents
	TotalExpense int // cents
	Balance      int // cents (positive = surplus, negative = deficit)
}

// TotalIncomeDollars returns income in dollars.
func (s *BudgetSummary) TotalIncomeDollars() float64 {
	return float64(s.TotalIncome) / 100.0
}

// TotalExpenseDollars returns expenses in dollars.
func (s *BudgetSummary) TotalExpenseDollars() float64 {
	return float64(s.TotalExpense) / 100.0
}

// BalanceDollars returns the balance in dollars.
func (s *BudgetSummary) BalanceDollars() float64 {
	return float64(s.Balance) / 100.0
}

// IsSurplus returns true if income >= expenses.
func (s *BudgetSummary) IsSurplus() bool {
	return s.Balance >= 0
}

// EventBudgetRow is one row in the yearly budget table.
type EventBudgetRow struct {
	EventID      string
	EventName    string
	Quarter      string
	TotalIncome  int // cents
	TotalExpense int // cents
	Balance      int // cents
}

// TotalIncomeDollars returns income in dollars.
func (r *EventBudgetRow) TotalIncomeDollars() float64 {
	return float64(r.TotalIncome) / 100.0
}

// TotalExpenseDollars returns expenses in dollars.
func (r *EventBudgetRow) TotalExpenseDollars() float64 {
	return float64(r.TotalExpense) / 100.0
}

// BalanceDollars returns the balance in dollars.
func (r *EventBudgetRow) BalanceDollars() float64 {
	return float64(r.Balance) / 100.0
}
