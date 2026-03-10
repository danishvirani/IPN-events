package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type ParticipantHandler struct {
	eventRepo       *db.EventRepository
	participantRepo *db.ParticipantRepository
}

func NewParticipantHandler(eventRepo *db.EventRepository, participantRepo *db.ParticipantRepository) *ParticipantHandler {
	return &ParticipantHandler{eventRepo: eventRepo, participantRepo: participantRepo}
}

// requireEventAccess loads event and checks admin-or-owner authorization.
// Returns the event or nil (and writes an error response) if unauthorized.
func (h *ParticipantHandler) requireEventAccess(w http.ResponseWriter, r *http.Request) (*models.Event, *models.User) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return nil, nil
	}

	if !user.IsAdmin() && e.UserID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return nil, nil
	}
	return e, user
}

// ── Admin-only endpoints ─────────────────────────────────────────────────────

type participantsPageData struct {
	Event        *models.Event
	Participants []*models.Participant
	Counts       *models.ParticipantCounts
	Search       string
}

// ListParticipants shows all participants for an event (admin only).
func (h *ParticipantHandler) ListParticipants(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	query := r.URL.Query().Get("q")
	var participants []*models.Participant
	if query != "" {
		participants, _ = h.participantRepo.SearchByEvent(id, query)
	} else {
		participants, _ = h.participantRepo.ListByEvent(id)
	}
	counts, _ := h.participantRepo.CountByEvent(id)

	render(w, r, "web/templates/admin/participants.html", participantsPageData{
		Event:        e,
		Participants: participants,
		Counts:       counts,
		Search:       query,
	})
}

// DownloadTemplate streams a CSV template for participant import.
func (h *ParticipantHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="participants_template.csv"`)

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"first_name", "last_name", "phone", "email", "jamatkhana", "gender", "company", "title"})
	writer.Flush()
}

// ImportCSV handles CSV upload and upsert of participants.
func (h *ParticipantHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		setFlash(w, "error", "File too large.")
		http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		setFlash(w, "error", "Please select a CSV file.")
		http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		setFlash(w, "error", "Could not read CSV header.")
		http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
		return
	}

	// Build column index map
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	getCol := func(row []string, name string) string {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
		return ""
	}

	var participants []*models.Participant
	lineNum := 1
	for {
		row, err := reader.Read()
		if err != nil {
			break
		}
		lineNum++

		// Accept both "title" and "role" column names
		title := getCol(row, "title")
		if title == "" {
			title = getCol(row, "role")
		}

		p := &models.Participant{
			FirstName:  getCol(row, "first_name"),
			LastName:   getCol(row, "last_name"),
			Phone:      getCol(row, "phone"),
			Email:      strings.ToLower(getCol(row, "email")),
			Jamatkhana: getCol(row, "jamatkhana"),
			Gender:     getCol(row, "gender"),
			Company:    getCol(row, "company"),
			Role:       title,
		}

		// Skip completely empty rows
		if p.FirstName == "" && p.LastName == "" && p.Email == "" && p.Phone == "" {
			continue
		}
		participants = append(participants, p)
	}

	if len(participants) == 0 {
		setFlash(w, "error", "No valid participant rows found in CSV.")
		http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
		return
	}

	created, updated, err := h.participantRepo.BulkUpsert(e.ID, participants)
	if err != nil {
		setFlash(w, "error", "Import failed: "+err.Error())
		http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Imported %d new", created)
	if updated > 0 {
		msg += fmt.Sprintf(", updated %d existing", updated)
	}
	msg += " participants."
	setFlash(w, "success", msg)
	http.Redirect(w, r, "/admin/events/"+e.ID+"/participants", http.StatusSeeOther)
}

// DeleteParticipant removes a participant (admin only).
func (h *ParticipantHandler) DeleteParticipant(w http.ResponseWriter, r *http.Request) {
	eventID := chi.URLParam(r, "id")
	pid := chi.URLParam(r, "pid")

	p, err := h.participantRepo.GetByID(pid)
	if err != nil || p.EventID != eventID {
		http.NotFound(w, r)
		return
	}

	_ = h.participantRepo.Delete(pid)
	setFlash(w, "success", "Participant removed.")
	http.Redirect(w, r, "/admin/events/"+eventID+"/participants", http.StatusSeeOther)
}

// TogglePaidEvent toggles the is_paid_event flag (admin only).
func (h *ParticipantHandler) TogglePaidEvent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	isPaid := r.FormValue("is_paid_event") == "1"
	_ = h.eventRepo.UpdateIsPaidEvent(id, isPaid)
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin/events/" + id
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// ── Check-in endpoints (admin or event owner) ───────────────────────────────

type checkinPageData struct {
	Event        *models.Event
	Participants []*models.Participant
	Counts       *models.ParticipantCounts
	Search       string
}

// CheckinPage shows the check-in page for an event.
func (h *ParticipantHandler) CheckinPage(w http.ResponseWriter, r *http.Request) {
	e, _ := h.requireEventAccess(w, r)
	if e == nil {
		return
	}

	query := r.URL.Query().Get("q")
	var participants []*models.Participant
	if query != "" {
		participants, _ = h.participantRepo.SearchByEventName(e.ID, query)
	} else {
		participants, _ = h.participantRepo.ListByEvent(e.ID)
	}
	counts, _ := h.participantRepo.CountByEvent(e.ID)

	render(w, r, "web/templates/admin/checkin.html", checkinPageData{
		Event:        e,
		Participants: participants,
		Counts:       counts,
		Search:       query,
	})
}

// ToggleCheckin toggles a participant's check-in status.
func (h *ParticipantHandler) ToggleCheckin(w http.ResponseWriter, r *http.Request) {
	e, _ := h.requireEventAccess(w, r)
	if e == nil {
		return
	}

	pid := chi.URLParam(r, "pid")
	p, err := h.participantRepo.GetByID(pid)
	if err != nil || p.EventID != e.ID {
		http.NotFound(w, r)
		return
	}

	_ = h.participantRepo.SetCheckedIn(pid, !p.CheckedIn)

	redirect := "/admin/events/" + e.ID + "/checkin"
	if q := r.URL.Query().Get("q"); q != "" {
		redirect += "?q=" + q
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// TogglePaid toggles a participant's paid status.
func (h *ParticipantHandler) TogglePaid(w http.ResponseWriter, r *http.Request) {
	e, _ := h.requireEventAccess(w, r)
	if e == nil {
		return
	}

	pid := chi.URLParam(r, "pid")
	p, err := h.participantRepo.GetByID(pid)
	if err != nil || p.EventID != e.ID {
		http.NotFound(w, r)
		return
	}

	_ = h.participantRepo.SetPaid(pid, !p.Paid)

	redirect := "/admin/events/" + e.ID + "/checkin"
	if q := r.URL.Query().Get("q"); q != "" {
		redirect += "?q=" + q
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// AddWalkin adds a walk-in participant (auto checked-in).
func (h *ParticipantHandler) AddWalkin(w http.ResponseWriter, r *http.Request) {
	e, _ := h.requireEventAccess(w, r)
	if e == nil {
		return
	}

	now := time.Now()
	p := &models.Participant{
		EventID:     e.ID,
		FirstName:   strings.TrimSpace(r.FormValue("first_name")),
		LastName:    strings.TrimSpace(r.FormValue("last_name")),
		Phone:       strings.TrimSpace(r.FormValue("phone")),
		Email:       strings.TrimSpace(strings.ToLower(r.FormValue("email"))),
		IsWalkin:    true,
		CheckedIn:   true,
		CheckedInAt: &now,
	}

	if p.FirstName == "" && p.LastName == "" {
		setFlash(w, "error", "Please enter at least a first or last name.")
		http.Redirect(w, r, "/admin/events/"+e.ID+"/checkin", http.StatusSeeOther)
		return
	}

	if err := h.participantRepo.Create(p); err != nil {
		setFlash(w, "error", "Could not add participant.")
	} else {
		setFlash(w, "success", p.FullName()+" added and checked in.")
	}
	http.Redirect(w, r, "/admin/events/"+e.ID+"/checkin", http.StatusSeeOther)
}

// ── Cross-event registrants (admin only) ─────────────────────────────────────

type registrantsPageData struct {
	Registrants []*models.Registrant
	Search      string
	Company     string
	Role        string
	Companies   []string
	Roles       []string
	Total       int
}

// ListRegistrants shows a unified, deduplicated view of registrants across all events.
func (h *ParticipantHandler) ListRegistrants(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	search := q.Get("q")
	company := q.Get("company")
	role := q.Get("role")

	registrants, err := h.participantRepo.ListAllRegistrants(search, company, role)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	companies, _ := h.participantRepo.ListDistinctCompanies()
	roles, _ := h.participantRepo.ListDistinctRoles()

	render(w, r, "web/templates/admin/registrants.html", registrantsPageData{
		Registrants: registrants,
		Search:      search,
		Company:     company,
		Role:        role,
		Companies:   companies,
		Roles:       roles,
		Total:       len(registrants),
	})
}

// ShowRegistrant shows a single registrant's detail page.
func (h *ParticipantHandler) ShowRegistrant(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	reg, err := h.participantRepo.GetRegistrantByKey(key)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	render(w, r, "web/templates/admin/registrant_show.html", reg)
}

// SetRegistrationMode toggles the registration mode for an event (admin only).
func (h *ParticipantHandler) SetRegistrationMode(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	mode := r.FormValue("registration_mode")
	if mode != "full" && mode != "count_only" {
		mode = "full"
	}
	_ = h.eventRepo.UpdateRegistrationMode(id, mode)
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin/events/" + id
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// UpdateAttendance saves the manual attendance count + registration/participation counts.
func (h *ParticipantHandler) UpdateAttendance(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	attendance, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("attendance_count")))
	registration, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("registration_count")))
	participation, _ := strconv.Atoi(strings.TrimSpace(r.FormValue("participation_count")))

	_ = h.eventRepo.UpdateAttendanceCount(id, attendance)
	_ = h.eventRepo.UpdateAttendance(id, registration, participation)

	setFlash(w, "success", "Counts updated.")
	referer := r.Header.Get("Referer")
	if referer == "" {
		referer = "/admin/events/" + id
	}
	http.Redirect(w, r, referer, http.StatusSeeOther)
}

// ExportParticipants exports all participants as CSV.
func (h *ParticipantHandler) ExportParticipants(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	participants, _ := h.participantRepo.ListByEvent(id)

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="participants_%s.csv"`, e.ID[:8]))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"First Name", "Last Name", "Email", "Phone", "Jamatkhana", "Gender", "Company", "Title", "Checked In", "Checked In At", "Paid", "Paid At"})

	for _, p := range participants {
		checkedIn := ""
		if p.CheckedIn {
			checkedIn = "Yes"
		}
		checkedInAt := ""
		if p.CheckedInAt != nil {
			checkedInAt = p.CheckedInAt.Format("2006-01-02 15:04")
		}
		paid := ""
		if p.Paid {
			paid = "Yes"
		}
		paidAt := ""
		if p.PaidAt != nil {
			paidAt = p.PaidAt.Format("2006-01-02 15:04")
		}

		_ = writer.Write([]string{
			p.FirstName, p.LastName, p.Email, p.Phone,
			p.Jamatkhana, p.Gender, p.Company, p.Role,
			checkedIn, checkedInAt, paid, paidAt,
		})
	}
	writer.Flush()
}
