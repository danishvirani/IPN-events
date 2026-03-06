package handlers

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

// BudgetHandler handles budget operations for events.
type BudgetHandler struct {
	eventRepo  *db.EventRepository
	budgetRepo *db.BudgetRepository
}

// NewBudgetHandler creates a new BudgetHandler.
func NewBudgetHandler(eventRepo *db.EventRepository, budgetRepo *db.BudgetRepository) *BudgetHandler {
	return &BudgetHandler{eventRepo: eventRepo, budgetRepo: budgetRepo}
}

// AddItem handles POST /admin/events/{id}/budget — adds a budget line item.
func (h *BudgetHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")

	// Verify event exists and is approved
	e, err := h.eventRepo.GetByID(eventID)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if e.Status != "approved" {
		http.Error(w, "Budget can only be added to approved events", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	itemType := r.FormValue("type")
	if itemType != models.BudgetTypeIncome && itemType != models.BudgetTypeExpense {
		http.Error(w, "Invalid type", http.StatusBadRequest)
		return
	}

	category := r.FormValue("category")
	if category == "" {
		setFlash(w, "error", "Category is required.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	description := strings.TrimSpace(r.FormValue("description"))

	// For "other" categories, description is required to specify what it is
	if (category == models.ExpenseCatOther || category == models.IncomeCatOther) && description == "" {
		setFlash(w, "error", "Please specify a description for 'Other' items.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	quantity, _ := strconv.Atoi(r.FormValue("quantity"))
	if quantity < 1 {
		quantity = 1
	}

	// Parse dollar amount and convert to cents
	amountStr := strings.TrimSpace(r.FormValue("unit_amount"))
	amountStr = strings.TrimPrefix(amountStr, "$")
	amountStr = strings.ReplaceAll(amountStr, ",", "")
	amountFloat, err := strconv.ParseFloat(amountStr, 64)
	if err != nil || amountFloat < 0 {
		setFlash(w, "error", "Please enter a valid amount.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}
	unitAmountCents := int(math.Round(amountFloat * 100))

	if unitAmountCents == 0 {
		setFlash(w, "error", "Amount must be greater than zero.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	item := &models.BudgetItem{
		EventID:     eventID,
		Type:        itemType,
		Category:    category,
		Description: description,
		Quantity:    quantity,
		UnitAmount:  unitAmountCents,
	}

	if err := h.budgetRepo.Create(item); err != nil {
		setFlash(w, "error", "Failed to add budget item.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Budget item added.")
	http.Redirect(w, r, "/admin/events/"+eventID+"#budget-section", http.StatusSeeOther)
}

// DeleteItem handles POST /admin/events/{id}/budget/{itemId}/delete — removes a budget item.
func (h *BudgetHandler) DeleteItem(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	itemID := chi.URLParam(r, "itemId")

	// Verify item belongs to this event
	item, err := h.budgetRepo.GetByID(itemID)
	if err != nil || item.EventID != eventID {
		http.NotFound(w, r)
		return
	}

	if err := h.budgetRepo.Delete(itemID); err != nil {
		setFlash(w, "error", "Failed to delete budget item.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Budget item removed.")
	http.Redirect(w, r, "/admin/events/"+eventID+"#budget-section", http.StatusSeeOther)
}

// YearlyOverview handles GET /admin/budget — shows aggregate budget for the year.
func (h *BudgetHandler) YearlyOverview(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2024 {
		year = time.Now().Year()
	}

	rows, err := h.budgetRepo.YearlySummary(year)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var totalIncome, totalExpense int
	for _, row := range rows {
		totalIncome += row.TotalIncome
		totalExpense += row.TotalExpense
	}

	data := struct {
		Year         int
		Rows         []models.EventBudgetRow
		TotalIncome  int
		TotalExpense int
		Balance      int
		EventCount   int
	}{
		Year:         year,
		Rows:         rows,
		TotalIncome:  totalIncome,
		TotalExpense: totalExpense,
		Balance:      totalIncome - totalExpense,
		EventCount:   len(rows),
	}

	render(w, r, "web/templates/admin/budget.html", data)
}
