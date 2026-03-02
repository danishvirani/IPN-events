package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type EventHandler struct {
	eventRepo *db.EventRepository
}

func NewEventHandler(eventRepo *db.EventRepository) *EventHandler {
	return &EventHandler{eventRepo: eventRepo}
}

func (h *EventHandler) New(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/events/new.html", nil)
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user := middleware.UserFromContext(r.Context())
	e := parseEventForm(r)
	e.UserID = user.ID

	if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Description) == "" {
		setFlash(w, "error", "Event name and description are required.")
		http.Redirect(w, r, "/events/new", http.StatusSeeOther)
		return
	}
	if e.Year == 0 {
		setFlash(w, "error", "Year is required.")
		http.Redirect(w, r, "/events/new", http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.Create(e); err != nil {
		setFlash(w, "error", "Failed to submit event. Please try again.")
		http.Redirect(w, r, "/events/new", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event submitted successfully!")
	http.Redirect(w, r, "/events/"+e.ID, http.StatusSeeOther)
}

func (h *EventHandler) ListMine(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	events, err := h.eventRepo.ListByUser(user.ID)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/events/list_member.html", events)
}

func (h *EventHandler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Team members can only view their own events
	if !user.IsAdmin() && e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	render(w, r, "web/templates/events/show.html", e)
}

func (h *EventHandler) Edit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if e.Status == models.StatusApproved {
		setFlash(w, "error", "Approved events cannot be edited.")
		http.Redirect(w, r, "/events/"+id, http.StatusSeeOther)
		return
	}

	render(w, r, "web/templates/events/edit.html", e)
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	if e.Status == models.StatusApproved {
		setFlash(w, "error", "Approved events cannot be edited.")
		http.Redirect(w, r, "/events/"+id, http.StatusSeeOther)
		return
	}

	updated := parseEventForm(r)
	updated.ID = id
	updated.UserID = user.ID

	if strings.TrimSpace(updated.Name) == "" || strings.TrimSpace(updated.Description) == "" {
		setFlash(w, "error", "Event name and description are required.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	if updated.Year == 0 {
		setFlash(w, "error", "Year is required.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.Update(updated); err != nil {
		setFlash(w, "error", "Failed to update event. Please try again.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event updated and resubmitted for review.")
	http.Redirect(w, r, "/events/"+id, http.StatusSeeOther)
}

// parseEventForm reads all event form fields from the request.
func parseEventForm(r *http.Request) *models.Event {
	year, _ := strconv.Atoi(r.FormValue("year"))

	recurrence := r.FormValue("recurrence")
	if recurrence == "" {
		recurrence = models.RecurrenceNone
	}

	scope := r.FormValue("scope")
	if scope == "" {
		scope = models.ScopeRegional
	}

	venueType := r.FormValue("venue_type")
	if venueType == "" {
		venueType = models.VenueTypeExternal
	}

	e := &models.Event{
		Name:              strings.TrimSpace(r.FormValue("name")),
		Quarter:           r.FormValue("quarter"),
		Year:              year,
		Recurrence:        recurrence,
		RecurrenceEndDate: strings.TrimSpace(r.FormValue("recurrence_end_date")),
		Description: strings.TrimSpace(r.FormValue("description")),
		City:        strings.TrimSpace(r.FormValue("city")),
		Scope:       scope,
		ScopeJamatkhana: r.FormValue("scope_jamatkhana"),
		VenueType:       venueType,
		VenueJamatkhana: r.FormValue("venue_jamatkhana"),
		VenueAddress:    strings.TrimSpace(r.FormValue("venue_address")),
		Outcome:     strings.TrimSpace(r.FormValue("outcome")),
		Impact:      strings.TrimSpace(r.FormValue("impact")),
		Input: models.EventInput{
			FinancialResources: strings.TrimSpace(r.FormValue("input_financial")),
			Facilities:         strings.TrimSpace(r.FormValue("input_facilities")),
			HumanSupport:       strings.TrimSpace(r.FormValue("input_human_support")),
			Technology:         strings.TrimSpace(r.FormValue("input_technology")),
			Partnerships:       strings.TrimSpace(r.FormValue("input_partnerships")),
		},
		Activities: models.EventActivities{
			StructuredProgramming: strings.TrimSpace(r.FormValue("activities_structured")),
			EngagementDesign:      strings.TrimSpace(r.FormValue("activities_engagement")),
			ContentDelivery:       strings.TrimSpace(r.FormValue("activities_content")),
			CommunityBuilding:     strings.TrimSpace(r.FormValue("activities_community")),
		},
	}

	// Parse output bullet items
	for i, desc := range r.Form["output_description"] {
		desc = strings.TrimSpace(desc)
		if desc == "" {
			continue
		}
		e.OutputItems = append(e.OutputItems, models.OutputItem{
			Description: desc,
			SortOrder:   i,
		})
	}

	// Parse support requests (parallel arrays)
	types := r.Form["support_type"]
	descs := r.Form["support_description"]
	venueTypes := r.Form["support_venue_type"]
	venueDetails := r.Form["support_venue_detail"]

	for i, t := range types {
		desc := ""
		if i < len(descs) {
			desc = strings.TrimSpace(descs[i])
		}
		vt := ""
		if i < len(venueTypes) {
			vt = venueTypes[i]
		}
		vd := ""
		if i < len(venueDetails) {
			vd = strings.TrimSpace(venueDetails[i])
		}
		e.SupportRequests = append(e.SupportRequests, models.SupportRequest{
			Type:        t,
			Description: desc,
			VenueType:   vt,
			VenueDetail: vd,
			SortOrder:   i,
		})
	}

	return e
}

// unused import guard
var _ = strconv.Itoa
