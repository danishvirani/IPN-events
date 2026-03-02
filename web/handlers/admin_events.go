package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

// ── Roadmap / quarterly structures ───────────────────────────────────────────

// calendarYear holds a year's events split by quarter for the roadmap view.
type calendarYear struct {
	Year        int
	Q1          []*models.Event
	Q2          []*models.Event
	Q3          []*models.Event
	Q4          []*models.Event
	Unscheduled []*models.Event
}

// ── 12-month Google Calendar structures ──────────────────────────────────────

// CalDayCell is a single cell in a month grid (Day=0 means empty padding).
type CalDayCell struct {
	Day    int
	Events []*models.Event
}

// CalMonth is one month's mini-calendar grid plus the events for that month.
type CalMonth struct {
	Name        string
	Month       int
	MonthEvents []*models.Event // events whose quarter starts this month
	Cells       []CalDayCell    // padded day cells (0 = empty)
}

// CalPageData is passed to calendar.html.
type CalPageData struct {
	Year        int
	PrevYear    int
	NextYear    int
	Months      []CalMonth
	Unscheduled []*models.Event
}

// quarterStartMonth maps a quarter tag to its first calendar month.
var quarterStartMonth = map[string]int{
	"Q1": 1, "Q2": 4, "Q3": 7, "Q4": 10,
}

var monthNames = [12]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// buildCalPage converts a flat event list into CalPageData for the given year.
func buildCalPage(events []*models.Event, year int) CalPageData {
	// Bucket events: month→day→events
	type key struct{ month, day int }
	byDate := map[key][]*models.Event{}
	var unscheduled []*models.Event

	for _, e := range events {
		if e.Year != year {
			continue
		}
		m, ok := quarterStartMonth[e.Quarter]
		if !ok {
			unscheduled = append(unscheduled, e)
			continue
		}
		k := key{m, 1}
		byDate[k] = append(byDate[k], e)
	}

	months := make([]CalMonth, 12)
	for m := 1; m <= 12; m++ {
		firstDay := time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		// days in month: day 0 of next month
		daysInMonth := time.Date(year, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
		offset := int(firstDay.Weekday()) // Sunday = 0

		cells := make([]CalDayCell, offset) // leading empty cells
		for d := 1; d <= daysInMonth; d++ {
			cells = append(cells, CalDayCell{
				Day:    d,
				Events: byDate[key{m, d}],
			})
		}
		// Pad to complete last row (multiple of 7)
		for len(cells)%7 != 0 {
			cells = append(cells, CalDayCell{})
		}

		months[m-1] = CalMonth{
			Name:        monthNames[m-1],
			Month:       m,
			MonthEvents: byDate[key{m, 1}],
			Cells:       cells,
		}
	}

	return CalPageData{
		Year:        year,
		PrevYear:    year - 1,
		NextYear:    year + 1,
		Months:      months,
		Unscheduled: unscheduled,
	}
}

// ── Handler ───────────────────────────────────────────────────────────────────

type AdminEventHandler struct {
	eventRepo *db.EventRepository
}

func NewAdminEventHandler(eventRepo *db.EventRepository) *AdminEventHandler {
	return &AdminEventHandler{eventRepo: eventRepo}
}

type adminEventListData struct {
	Events       interface{}
	StatusFilter string
}

func (h *AdminEventHandler) List(w http.ResponseWriter, r *http.Request) {
	statusFilter := r.URL.Query().Get("status")
	events, err := h.eventRepo.ListAll(statusFilter)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/events/list_admin.html", adminEventListData{
		Events:       events,
		StatusFilter: statusFilter,
	})
}

func (h *AdminEventHandler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "web/templates/events/show.html", e)
}

// ReviewForm returns the approve/reject partial for HTMX loading.
func (h *AdminEventHandler) ReviewForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	renderPartial(w, r, "web/templates/partials/review_form.html", e)
}

func (h *AdminEventHandler) Approve(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	comment := strings.TrimSpace(r.FormValue("admin_comment"))

	if err := h.eventRepo.Approve(id, comment); err != nil {
		setFlash(w, "error", "Failed to approve event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event approved.")
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

func (h *AdminEventHandler) Calendar(w http.ResponseWriter, r *http.Request) {
	yearStr := r.URL.Query().Get("year")
	year, err := strconv.Atoi(yearStr)
	if err != nil || year < 2000 || year > 2100 {
		year = time.Now().Year()
	}

	events, err := h.eventRepo.ListForCalendar()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/calendar.html", buildCalPage(events, year))
}

func (h *AdminEventHandler) Roadmap(w http.ResponseWriter, r *http.Request) {
	events, err := h.eventRepo.ListForCalendar()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/roadmap.html", groupByYear(events))
}

// groupByYear organises a flat event list into calendarYear structs.
func groupByYear(events []*models.Event) []calendarYear {
	index := map[int]*calendarYear{}
	var order []int

	for _, e := range events {
		y := e.Year
		if _, ok := index[y]; !ok {
			index[y] = &calendarYear{Year: y}
			order = append(order, y)
		}
		cy := index[y]
		switch e.Quarter {
		case "Q1":
			cy.Q1 = append(cy.Q1, e)
		case "Q2":
			cy.Q2 = append(cy.Q2, e)
		case "Q3":
			cy.Q3 = append(cy.Q3, e)
		case "Q4":
			cy.Q4 = append(cy.Q4, e)
		default:
			cy.Unscheduled = append(cy.Unscheduled, e)
		}
	}

	rows := make([]calendarYear, 0, len(order))
	for _, y := range order {
		rows = append(rows, *index[y])
	}
	return rows
}

func (h *AdminEventHandler) Reject(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	comment := strings.TrimSpace(r.FormValue("admin_comment"))

	if comment == "" {
		setFlash(w, "error", "A comment is required when rejecting an event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.Reject(id, comment); err != nil {
		setFlash(w, "error", "Failed to reject event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event rejected with feedback.")
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

// SetDate allows an admin to set or clear the specific event date on an approved event.
func (h *AdminEventHandler) SetDate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if e.Status != models.StatusApproved {
		setFlash(w, "error", "Event date can only be set on approved events.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	date := strings.TrimSpace(r.FormValue("event_date"))
	if date != "" {
		if _, err := time.Parse("2006-01-02", date); err != nil {
			setFlash(w, "error", "Invalid date format. Use YYYY-MM-DD.")
			http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
			return
		}
	}

	if err := h.eventRepo.SetEventDate(id, date); err != nil {
		setFlash(w, "error", "Failed to set event date.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	if date == "" {
		setFlash(w, "success", "Event date cleared.")
	} else {
		setFlash(w, "success", "Event date set to "+date+".")
	}
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

// DownloadTemplate streams a CSV template file with the required column headers.
func (h *AdminEventHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="events_template.csv"`)

	headers := []string{
		"name", "quarter", "year", "description", "recurrence",
		"outcome", "impact",
		"financial_resources", "facilities", "human_support", "technology", "partnerships",
		"structured_programming", "engagement_design", "content_delivery", "community_building",
		"event_date",
	}
	writer := csv.NewWriter(w)
	_ = writer.Write(headers)
	writer.Flush()
}

// ImportPage renders the CSV import upload page.
func (h *AdminEventHandler) ImportPage(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/admin/import.html", nil)
}

// ImportCSV parses an uploaded CSV file and bulk-creates events as approved.
func (h *AdminEventHandler) ImportCSV(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		setFlash(w, "error", "Failed to parse upload.")
		http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
		return
	}

	file, _, err := r.FormFile("csv_file")
	if err != nil {
		setFlash(w, "error", "No file uploaded.")
		http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
		return
	}
	defer file.Close()

	adminUser := middleware.UserFromContext(r.Context())

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	headers, err := reader.Read()
	if err != nil {
		setFlash(w, "error", "Could not read CSV header row.")
		http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
		return
	}

	colIdx := map[string]int{}
	for i, h := range headers {
		colIdx[strings.TrimSpace(h)] = i
	}

	for _, col := range []string{"name", "year", "description"} {
		if _, ok := colIdx[col]; !ok {
			setFlash(w, "error", "CSV is missing required column: "+col)
			http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
			return
		}
	}

	getCol := func(row []string, name string) string {
		i, ok := colIdx[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}

	var events []*models.Event
	lineNum := 1
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			setFlash(w, "error", fmt.Sprintf("CSV parse error on line %d: %v", lineNum+1, err))
			http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
			return
		}
		lineNum++

		yearInt, _ := strconv.Atoi(getCol(row, "year"))
		recurrence := getCol(row, "recurrence")
		if recurrence == "" {
			recurrence = models.RecurrenceNone
		}

		e := &models.Event{
			UserID:      adminUser.ID,
			Name:        getCol(row, "name"),
			Quarter:     getCol(row, "quarter"),
			Year:        yearInt,
			Recurrence:  recurrence,
			Description: getCol(row, "description"),
			Outcome:     getCol(row, "outcome"),
			Impact:      getCol(row, "impact"),
			EventDate:   getCol(row, "event_date"),
			Input: models.EventInput{
				FinancialResources: getCol(row, "financial_resources"),
				Facilities:         getCol(row, "facilities"),
				HumanSupport:       getCol(row, "human_support"),
				Technology:         getCol(row, "technology"),
				Partnerships:       getCol(row, "partnerships"),
			},
			Activities: models.EventActivities{
				StructuredProgramming: getCol(row, "structured_programming"),
				EngagementDesign:      getCol(row, "engagement_design"),
				ContentDelivery:       getCol(row, "content_delivery"),
				CommunityBuilding:     getCol(row, "community_building"),
			},
		}

		if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Description) == "" || e.Year == 0 {
			setFlash(w, "error", fmt.Sprintf("Row %d: name, description, and year are required.", lineNum))
			http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
			return
		}

		events = append(events, e)
	}

	if len(events) == 0 {
		setFlash(w, "error", "CSV contained no data rows.")
		http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.BulkCreate(events); err != nil {
		setFlash(w, "error", "Failed to import events: "+err.Error())
		http.Redirect(w, r, "/admin/events/import", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", fmt.Sprintf("Successfully imported %d event(s).", len(events)))
	http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
}
