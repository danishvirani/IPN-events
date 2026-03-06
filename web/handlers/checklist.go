package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type ChecklistHandler struct {
	eventRepo     *db.EventRepository
	checklistRepo *db.ChecklistRepository
}

func NewChecklistHandler(eventRepo *db.EventRepository, checklistRepo *db.ChecklistRepository) *ChecklistHandler {
	return &ChecklistHandler{eventRepo: eventRepo, checklistRepo: checklistRepo}
}

// ToggleItem handles POST /admin/events/{id}/checklist/toggle
func (h *ChecklistHandler) ToggleItem(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	itemKey := r.FormValue("item_key")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(eventID)
	if err != nil || e.Status != "approved" {
		http.NotFound(w, r)
		return
	}

	if err := h.checklistRepo.ToggleStatus(eventID, itemKey, user.ID); err != nil {
		setFlash(w, "error", "Failed to update checklist item.")
	}

	http.Redirect(w, r, "/admin/events/"+eventID+"#checklist-section", http.StatusSeeOther)
}

// AddItem handles POST /admin/events/{id}/checklist/add
func (h *ChecklistHandler) AddItem(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	itemKey := r.FormValue("item_key")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(eventID)
	if err != nil || e.Status != "approved" {
		http.NotFound(w, r)
		return
	}

	if !isValidSelectableItem(itemKey) {
		setFlash(w, "error", "Invalid checklist item.")
		http.Redirect(w, r, "/admin/events/"+eventID, http.StatusSeeOther)
		return
	}

	if err := h.checklistRepo.AddItem(eventID, itemKey, user.ID); err != nil {
		setFlash(w, "error", "Failed to add checklist item.")
	}

	http.Redirect(w, r, "/admin/events/"+eventID+"#checklist-section", http.StatusSeeOther)
}

// RemoveItem handles POST /admin/events/{id}/checklist/remove
func (h *ChecklistHandler) RemoveItem(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	itemKey := r.FormValue("item_key")

	e, err := h.eventRepo.GetByID(eventID)
	if err != nil || e.Status != "approved" {
		http.NotFound(w, r)
		return
	}

	if err := h.checklistRepo.RemoveItem(eventID, itemKey); err != nil {
		setFlash(w, "error", "Failed to remove checklist item.")
	}

	http.Redirect(w, r, "/admin/events/"+eventID+"#checklist-section", http.StatusSeeOther)
}

// isValidSelectableItem checks that a key belongs to a selectable group.
func isValidSelectableItem(key string) bool {
	for _, def := range models.ChecklistCatalog {
		if def.Key == key {
			for _, g := range models.ChecklistGroups() {
				if g.GroupKey == def.Group && g.Selectable {
					return true
				}
			}
		}
	}
	return false
}

// BuildChecklistData constructs the grouped checklist view data for the show template.
func BuildChecklistData(event *models.Event, items []models.ChecklistItem, isAdmin bool) []models.ChecklistGroupData {
	// Index active items by key
	activeByKey := map[string]models.ChecklistItem{}
	for _, item := range items {
		activeByKey[item.ItemKey] = item
	}

	var groups []models.ChecklistGroupData
	for _, gdef := range models.ChecklistGroups() {
		catalogItems := models.CatalogByGroup(gdef.GroupKey)
		var viewItems []models.ChecklistItemView

		for _, def := range catalogItems {
			// Check condition: skip Space Reservation if venue is not Jamatkhana
			if def.Condition == "venue_jamatkhana" && event.VenueType != models.VenueTypeInternal {
				continue
			}

			active := false
			status := models.ChecklistStatusPending
			id := ""
			if item, ok := activeByKey[def.Key]; ok {
				active = true
				status = item.Status
				id = item.ID
			}

			// For non-selectable groups, only show items that are active
			// For selectable groups, show all items so admin can toggle them on/off
			if !gdef.Selectable && !active {
				continue
			}

			// Non-admin users: only show active items
			if !isAdmin && !active {
				continue
			}

			viewItems = append(viewItems, models.ChecklistItemView{
				Def:    def,
				Active: active,
				Status: status,
				ID:     id,
			})
		}

		// Skip empty groups entirely
		if len(viewItems) == 0 {
			continue
		}

		groups = append(groups, models.ChecklistGroupData{
			Label:      gdef.Label,
			GroupKey:    gdef.GroupKey,
			Items:      viewItems,
			Selectable: gdef.Selectable,
		})
	}
	return groups
}
