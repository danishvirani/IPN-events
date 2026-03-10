package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

// BudgetRepository handles CRUD for event budget items.
type BudgetRepository struct {
	db *sql.DB
}

// NewBudgetRepository creates a new BudgetRepository.
func NewBudgetRepository(db *sql.DB) *BudgetRepository {
	return &BudgetRepository{db: db}
}

// Create inserts a new budget item.
func (r *BudgetRepository) Create(item *models.BudgetItem) error {
	item.ID = uuid.New().String()
	item.CreatedAt = time.Now()

	// Get next sort order for this event+type
	var maxSort sql.NullInt64
	_ = r.db.QueryRow(
		`SELECT MAX(sort_order) FROM event_budget_items WHERE event_id = ? AND type = ?`,
		item.EventID, item.Type,
	).Scan(&maxSort)
	item.SortOrder = int(maxSort.Int64) + 1

	_, err := r.db.Exec(
		`INSERT INTO event_budget_items (id, event_id, type, category, description, quantity, unit_amount, sort_order, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.EventID, item.Type, item.Category,
		nullIfEmpty(item.Description),
		item.Quantity, item.UnitAmount, item.SortOrder, item.CreatedAt,
	)
	return err
}

// Delete removes a single budget item.
func (r *BudgetRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM event_budget_items WHERE id = ?`, id)
	return err
}

// GetByID fetches a single budget item.
func (r *BudgetRepository) GetByID(id string) (*models.BudgetItem, error) {
	item := &models.BudgetItem{}
	var desc sql.NullString
	err := r.db.QueryRow(
		`SELECT id, event_id, type, category, description, quantity, unit_amount, sort_order, created_at
		 FROM event_budget_items WHERE id = ?`, id,
	).Scan(&item.ID, &item.EventID, &item.Type, &item.Category, &desc,
		&item.Quantity, &item.UnitAmount, &item.SortOrder, &item.CreatedAt)
	if err != nil {
		return nil, err
	}
	item.Description = desc.String
	return item, nil
}

// ListByEvent returns all budget items for an event as a BudgetSummary.
func (r *BudgetRepository) ListByEvent(eventID string) (*models.BudgetSummary, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, type, category, description, quantity, unit_amount, sort_order, created_at
		 FROM event_budget_items
		 WHERE event_id = ?
		 ORDER BY type DESC, sort_order ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	summary := &models.BudgetSummary{}
	for rows.Next() {
		var item models.BudgetItem
		var desc sql.NullString
		if err := rows.Scan(&item.ID, &item.EventID, &item.Type, &item.Category, &desc,
			&item.Quantity, &item.UnitAmount, &item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Description = desc.String

		total := item.Total()
		switch item.Type {
		case models.BudgetTypeIncome:
			summary.IncomeItems = append(summary.IncomeItems, item)
			summary.TotalIncome += total
		case models.BudgetTypeExpense:
			summary.ExpenseItems = append(summary.ExpenseItems, item)
			summary.TotalExpense += total
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	summary.Balance = summary.TotalIncome - summary.TotalExpense
	return summary, nil
}

// CurrentYearBalance returns total income and expense (in cents) across all approved events for the current year.
func (r *BudgetRepository) CurrentYearBalance() (income, expense int, err error) {
	err = r.db.QueryRow(`
		SELECT
			COALESCE(SUM(CASE WHEN b.type='income' THEN b.quantity*b.unit_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN b.type='expense' THEN b.quantity*b.unit_amount ELSE 0 END), 0)
		FROM event_budget_items b
		JOIN events e ON b.event_id = e.id
		WHERE e.status = 'approved' AND e.year = CAST(strftime('%Y', 'now') AS INTEGER)`,
	).Scan(&income, &expense)
	return
}

// YearlySummary returns per-event budget totals for all approved events in a given year.
func (r *BudgetRepository) YearlySummary(year int) ([]models.EventBudgetRow, error) {
	rows, err := r.db.Query(`
		SELECT e.id, e.name, COALESCE(e.quarter, ''),
			COALESCE(SUM(CASE WHEN b.type = 'income' THEN b.quantity * b.unit_amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN b.type = 'expense' THEN b.quantity * b.unit_amount ELSE 0 END), 0)
		FROM events e
		LEFT JOIN event_budget_items b ON e.id = b.event_id
		WHERE e.status = 'approved' AND e.year = ?
		GROUP BY e.id
		ORDER BY e.quarter, e.name`,
		year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.EventBudgetRow
	for rows.Next() {
		var row models.EventBudgetRow
		if err := rows.Scan(&row.EventID, &row.EventName, &row.Quarter,
			&row.TotalIncome, &row.TotalExpense); err != nil {
			return nil, err
		}
		row.Balance = row.TotalIncome - row.TotalExpense
		result = append(result, row)
	}
	return result, rows.Err()
}
