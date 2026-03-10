package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

// EventShowData holds data for the event show template (used by both team and admin handlers).
type EventShowData struct {
	Event             *models.Event
	Comments          []*models.EventComment
	Budget            *models.BudgetSummary
	Checklist         []models.ChecklistGroupData
	TeamMembers       []*models.TeamMember
	AllUsers          []*models.User // for admin "Assigned To" dropdown
	ParticipantCounts *models.ParticipantCounts
}

// EventFormData holds data for the event new/edit form templates.
type EventFormData struct {
	Event       *models.Event
	Initiatives []*models.Initiative
}

type EventHandler struct {
	eventRepo       *db.EventRepository
	commentRepo     *db.CommentRepository
	initiativeRepo  *db.InitiativeRepository
	budgetRepo      *db.BudgetRepository
	checklistRepo   *db.ChecklistRepository
	teamRepo        *db.TeamRepository
	participantRepo *db.ParticipantRepository
	uploadDir       string
}

func NewEventHandler(eventRepo *db.EventRepository, commentRepo *db.CommentRepository, initiativeRepo *db.InitiativeRepository, budgetRepo *db.BudgetRepository, checklistRepo *db.ChecklistRepository, teamRepo *db.TeamRepository, participantRepo *db.ParticipantRepository, uploadDir string) *EventHandler {
	return &EventHandler{eventRepo: eventRepo, commentRepo: commentRepo, initiativeRepo: initiativeRepo, budgetRepo: budgetRepo, checklistRepo: checklistRepo, teamRepo: teamRepo, participantRepo: participantRepo, uploadDir: uploadDir}
}

// saveImage handles image upload from a multipart form field named "image".
// Returns the saved filename (relative), or keepExisting if no new file was uploaded.
func (h *EventHandler) saveImage(r *http.Request, keepExisting string) (string, error) {
	file, header, err := r.FormFile("image")
	if err != nil {
		// No file uploaded — keep existing image
		return keepExisting, nil
	}
	defer file.Close()

	if header.Size > 8<<20 {
		return "", fmt.Errorf("image must be under 8 MB")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp":
	default:
		return "", fmt.Errorf("unsupported image type; use JPG, PNG, GIF, or WebP")
	}

	filename := uuid.New().String() + ext
	dst, err := os.Create(filepath.Join(h.uploadDir, filename))
	if err != nil {
		return "", fmt.Errorf("could not save image")
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("could not save image")
	}
	return filename, nil
}

func (h *EventHandler) New(w http.ResponseWriter, r *http.Request) {
	initiatives, _ := h.initiativeRepo.ListAll()
	render(w, r, "web/templates/events/new.html", EventFormData{
		Event:       &models.Event{},
		Initiatives: initiatives,
	})
}

func (h *EventHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	user := middleware.UserFromContext(r.Context())
	e := parseEventForm(r)
	e.UserID = user.ID

	imagePath, err := h.saveImage(r, "")
	if err != nil {
		setFlash(w, "error", err.Error())
		http.Redirect(w, r, "/events/new", http.StatusSeeOther)
		return
	}
	e.ImagePath = imagePath

	if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Description) == "" {
		setFlash(w, "error", "Event name and description are required.")
		http.Redirect(w, r, "/events/new", http.StatusSeeOther)
		return
	}
	if e.Quarter == "" {
		setFlash(w, "error", "Quarter is required.")
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

	comments, _ := h.commentRepo.ListByEvent(id)

	var budget *models.BudgetSummary
	var checklistData []models.ChecklistGroupData
	if e.Status == "approved" {
		budget, _ = h.budgetRepo.ListByEvent(id)
		// Ensure default checklist items exist (idempotent — uses INSERT OR IGNORE)
		conditionFn := func(cond string) bool {
			if cond == "venue_jamatkhana" {
				return e.VenueType == models.VenueTypeInternal
			}
			return false
		}
		_ = h.checklistRepo.InitializeDefaults(id, "", conditionFn)
		clItems, _ := h.checklistRepo.ListByEvent(id)
		checklistData = BuildChecklistData(e, clItems, user.IsAdmin())
	}

	teamMembers, _ := h.teamRepo.ListByEvent(id)
	participantCounts, _ := h.participantRepo.CountByEvent(id)

	render(w, r, "web/templates/events/show.html", EventShowData{
		Event:             e,
		Comments:          comments,
		Budget:            budget,
		Checklist:         checklistData,
		TeamMembers:       teamMembers,
		ParticipantCounts: participantCounts,
	})
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

	initiatives, _ := h.initiativeRepo.ListAll()
	render(w, r, "web/templates/events/edit.html", EventFormData{
		Event:       e,
		Initiatives: initiatives,
	})
}

func (h *EventHandler) Update(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
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

	// Preserve existing image; replace if a new one was uploaded
	imagePath, err := h.saveImage(r, e.ImagePath)
	if err != nil {
		setFlash(w, "error", err.Error())
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	updated.ImagePath = imagePath

	if strings.TrimSpace(updated.Name) == "" || strings.TrimSpace(updated.Description) == "" {
		setFlash(w, "error", "Event name and description are required.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	if updated.Quarter == "" {
		setFlash(w, "error", "Quarter is required.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	if updated.Year == 0 {
		setFlash(w, "error", "Year is required.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	previousStatus := e.Status

	if err := h.eventRepo.Update(updated); err != nil {
		setFlash(w, "error", "Failed to update event. Please try again.")
		http.Redirect(w, r, "/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	// If the event was previously rejected, log a resubmit entry in the chat log
	if previousStatus == models.StatusRejected {
		resubmitText := strings.TrimSpace(r.FormValue("resubmit_note"))
		if resubmitText == "" {
			resubmitText = "Event edited and resubmitted for review."
		}
		_ = h.commentRepo.Create(&models.EventComment{
			EventID:  id,
			UserID:   user.ID,
			UserName: user.Name,
			Comment:  resubmitText,
			Type:     models.CommentTypeResubmit,
		})
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
		EventDate:         strings.TrimSpace(r.FormValue("event_date")),
		StartTime:         strings.TrimSpace(r.FormValue("start_time")),
		EndTime:           strings.TrimSpace(r.FormValue("end_time")),
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

	// Parse initiative IDs (checkboxes)
	e.InitiativeIDs = r.Form["initiative_ids"]

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

// AddComment lets a team member post a comment on their own event.
func (h *EventHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	text := strings.TrimSpace(r.FormValue("comment"))
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

	if text == "" {
		setFlash(w, "error", "Comment cannot be empty.")
		http.Redirect(w, r, "/events/"+id, http.StatusSeeOther)
		return
	}

	_ = h.commentRepo.Create(&models.EventComment{
		EventID:  id,
		UserID:   user.ID,
		UserName: user.Name,
		Comment:  text,
		Type:     models.CommentTypeComment,
	})

	http.Redirect(w, r, "/events/"+id, http.StatusSeeOther)
}

// unused import guard
var _ = strconv.Itoa
