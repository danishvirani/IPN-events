package handlers

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-pdf/fpdf"

	"ipn-events/internal/db"
	"ipn-events/internal/email"
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
	Events []*models.Event // only events with event_date on this exact day
}

// CalEvent wraps an event with an optional date label for calendar display.
type CalEvent struct {
	Event     *models.Event
	DateLabel string // "Jan 18" if event_date falls in this specific month; empty otherwise
}

// CalMonthBar represents a recurring event shown as a colored bar inside a month card.
type CalMonthBar struct {
	Event      *models.Event
	ColorClass string // Tailwind bg+text classes
}

// CalMonth is one month's mini-calendar grid plus recurring-event bars.
type CalMonth struct {
	Name        string
	Month       int
	MonthEvents []CalEvent // kept for compatibility; not rendered in current template
	Bars        []CalMonthBar
	Cells       []CalDayCell
}

// barColorClass returns a deterministic Tailwind color class pair for a recurring event.
func barColorClass(eventID string) string {
	palette := []string{
		"bg-purple-100 text-purple-700",
		"bg-emerald-100 text-emerald-700",
		"bg-orange-100 text-orange-700",
		"bg-rose-100 text-rose-700",
		"bg-sky-100 text-sky-700",
		"bg-amber-100 text-amber-700",
		"bg-violet-100 text-violet-700",
		"bg-teal-100 text-teal-700",
	}
	sum := 0
	for _, c := range eventID {
		sum += int(c)
	}
	return palette[sum%len(palette)]
}

// CalQuarterGroup holds the 3 month cards for a quarter plus all events for
// that quarter shown in a unified full-width list below the month grids.
type CalQuarterGroup struct {
	QuarterLabel string     // "Q1", "Q2", "Q3", "Q4"
	Months       []CalMonth // exactly 3 months
	Events       []CalEvent // all events for this quarter, sorted by date
}

// CalPageData is passed to calendar.html.
type CalPageData struct {
	Year          int
	PrevYear      int
	NextYear      int
	QuarterGroups []CalQuarterGroup
	Unscheduled   []*models.Event
}

// quarterStartMonth maps a quarter tag to its first calendar month.
var quarterStartMonth = map[string]int{
	"Q1": 1, "Q2": 4, "Q3": 7, "Q4": 10,
}

var monthNames = [12]string{
	"January", "February", "March", "April", "May", "June",
	"July", "August", "September", "October", "November", "December",
}

// eventMonthsForYear returns the list of months (1-12) this event is active in for the given year.
// Returns nil for events that don't belong to this year or have no quarter.
func eventMonthsForYear(e *models.Event, year int) []int {
	if e.Year != year {
		return nil
	}

	// Non-recurring or annual event with a specific date: use that date's month.
	// Annual events occur only once per year so their event_date determines the month.
	if e.EventDate != "" && (e.Recurrence == "" || e.Recurrence == models.RecurrenceNone || e.Recurrence == models.RecurrenceAnnual) {
		t, err := time.Parse("2006-01-02", e.EventDate)
		if err == nil && t.Year() == year {
			return []int{int(t.Month())}
		}
	}

	startMonth, hasQuarter := quarterStartMonth[e.Quarter]
	if !hasQuarter {
		return nil // unscheduled
	}

	// Refine start month using EventDate if it falls in this calendar year.
	if e.EventDate != "" {
		if t, err := time.Parse("2006-01-02", e.EventDate); err == nil && t.Year() == year {
			startMonth = int(t.Month())
		}
	}

	// Determine end month.
	// If no RecurrenceEndDate is set, default to 1 year from the event's start date.
	endMonth := 12
	if e.RecurrenceEndDate != "" {
		t, err := time.Parse("2006-01-02", e.RecurrenceEndDate)
		if err == nil {
			if t.Year() < year {
				return nil // already ended
			}
			if t.Year() == year {
				endMonth = int(t.Month())
			}
			// t.Year() > year → runs full year, endMonth stays 12
		}
	} else {
		// Default: run for 1 year from EventDate (or quarter start if no EventDate).
		var startRef time.Time
		if e.EventDate != "" {
			startRef, _ = time.Parse("2006-01-02", e.EventDate)
		}
		if startRef.IsZero() {
			startRef = time.Date(year, time.Month(startMonth), 1, 0, 0, 0, 0, time.UTC)
		}
		defaultEnd := startRef.AddDate(1, 0, 0)
		if defaultEnd.Year() < year {
			return nil // ended before this calendar year
		}
		if defaultEnd.Year() == year {
			endMonth = int(defaultEnd.Month())
		}
		// defaultEnd.Year() > year → event runs through end of this year
	}

	switch e.Recurrence {
	case models.RecurrenceMonthly, models.RecurrenceWeekly, models.RecurrenceBiWeekly:
		months := make([]int, 0, endMonth-startMonth+1)
		for m := startMonth; m <= endMonth; m++ {
			months = append(months, m)
		}
		return months
	case models.RecurrenceQuarterly:
		var months []int
		for _, m := range []int{1, 4, 7, 10} {
			if m >= startMonth && m <= endMonth {
				months = append(months, m)
			}
		}
		return months
	default: // none, annual
		return []int{startMonth}
	}
}

// buildCalPage converts a flat event list into CalPageData for the given year.
func buildCalPage(events []*models.Event, year int) CalPageData {
	type dayKey struct{ month, day int }

	// byDate: events with a specific event_date → for day-grid dot highlighting
	byDate := map[dayKey][]*models.Event{}
	// quarterEventsList: ALL events grouped by quarter → shown in the unified quarter list
	quarterEventsList := map[string][]CalEvent{}
	var unscheduled []*models.Event

	for _, e := range events {
		if e.Year != year {
			continue
		}

		// Grid dot: highlight the specific date on the mini-calendar
		if e.EventDate != "" {
			t, err := time.Parse("2006-01-02", e.EventDate)
			if err == nil && t.Year() == year {
				k := dayKey{int(t.Month()), t.Day()}
				byDate[k] = append(byDate[k], e)
			}
		}

		// Route event to its quarter list, or unscheduled if no quarter
		_, hasQ := quarterStartMonth[e.Quarter]
		if !hasQ {
			unscheduled = append(unscheduled, e)
			continue
		}

		dateLabel := ""
		if e.EventDate != "" {
			t, err := time.Parse("2006-01-02", e.EventDate)
			if err == nil && t.Year() == year {
				dateLabel = fmt.Sprintf("%s %d", monthNames[int(t.Month())-1][:3], t.Day())
			}
		}
		quarterEventsList[e.Quarter] = append(quarterEventsList[e.Quarter], CalEvent{Event: e, DateLabel: dateLabel})
	}

	// Sort each quarter's events: dated first (by date), undated last (by name)
	for q := range quarterEventsList {
		evs := quarterEventsList[q]
		sort.Slice(evs, func(i, j int) bool {
			di, dj := evs[i].Event.EventDate, evs[j].Event.EventDate
			if di == "" && dj == "" {
				return evs[i].Event.Name < evs[j].Event.Name
			}
			if di == "" {
				return false
			}
			if dj == "" {
				return true
			}
			return di < dj
		})
		quarterEventsList[q] = evs
	}

	// Build all 12 month structs (grid only — events listed at quarter level)
	allMonths := make([]CalMonth, 12)
	for m := 1; m <= 12; m++ {
		firstDay := time.Date(year, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
		daysInMonth := time.Date(year, time.Month(m+1), 0, 0, 0, 0, 0, time.UTC).Day()
		offset := int(firstDay.Weekday()) // Sunday = 0

		cells := make([]CalDayCell, offset)
		for d := 1; d <= daysInMonth; d++ {
			cells = append(cells, CalDayCell{
				Day:    d,
				Events: byDate[dayKey{m, d}],
			})
		}
		for len(cells)%7 != 0 {
			cells = append(cells, CalDayCell{})
		}

		allMonths[m-1] = CalMonth{
			Name:  monthNames[m-1],
			Month: m,
			Cells: cells,
		}
	}

	// Populate recurring-event bars for each month they are active.
	for _, e := range events {
		if e.Year != year {
			continue
		}
		switch e.Recurrence {
		case models.RecurrenceWeekly, models.RecurrenceBiWeekly, models.RecurrenceMonthly, models.RecurrenceQuarterly:
			color := barColorClass(e.ID)
			for _, m := range eventMonthsForYear(e, year) {
				allMonths[m-1].Bars = append(allMonths[m-1].Bars, CalMonthBar{Event: e, ColorClass: color})
			}
		}
	}

	// Group months into quarters
	quarterDefs := []struct {
		label  string
		months [3]int
	}{
		{"Q1", [3]int{1, 2, 3}},
		{"Q2", [3]int{4, 5, 6}},
		{"Q3", [3]int{7, 8, 9}},
		{"Q4", [3]int{10, 11, 12}},
	}
	quarterGroups := make([]CalQuarterGroup, 4)
	for i, qd := range quarterDefs {
		quarterGroups[i] = CalQuarterGroup{
			QuarterLabel: qd.label,
			Months: []CalMonth{
				allMonths[qd.months[0]-1],
				allMonths[qd.months[1]-1],
				allMonths[qd.months[2]-1],
			},
			Events: quarterEventsList[qd.label],
		}
	}

	return CalPageData{
		Year:          year,
		PrevYear:      year - 1,
		NextYear:      year + 1,
		QuarterGroups: quarterGroups,
		Unscheduled:   unscheduled,
	}
}

// ── Handler ───────────────────────────────────────────────────────────────────

type AdminEventHandler struct {
	eventRepo      *db.EventRepository
	commentRepo    *db.CommentRepository
	initiativeRepo *db.InitiativeRepository
	budgetRepo     *db.BudgetRepository
	checklistRepo  *db.ChecklistRepository
	emailSvc       *email.Service
	uploadDir      string
}

func NewAdminEventHandler(eventRepo *db.EventRepository, commentRepo *db.CommentRepository, initiativeRepo *db.InitiativeRepository, budgetRepo *db.BudgetRepository, checklistRepo *db.ChecklistRepository, emailSvc *email.Service, uploadDir string) *AdminEventHandler {
	return &AdminEventHandler{eventRepo: eventRepo, commentRepo: commentRepo, initiativeRepo: initiativeRepo, budgetRepo: budgetRepo, checklistRepo: checklistRepo, emailSvc: emailSvc, uploadDir: uploadDir}
}

// adminSaveImage is the same image-upload helper as EventHandler.saveImage but for admin handlers.
func (h *AdminEventHandler) adminSaveImage(r *http.Request, keepExisting string) (string, error) {
	file, header, err := r.FormFile("image")
	if err != nil {
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

	filename := randomHex(16) + ext
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

type adminEventListData struct {
	Events       []*models.Event
	StatusFilter string
	Search       string
	InitiativeID string
	Quarter      string
	Year         int
	Initiatives  []*models.Initiative
}

func (h *AdminEventHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	yearInt, _ := strconv.Atoi(q.Get("year"))

	filter := db.EventFilter{
		Status:       q.Get("status"),
		Search:       q.Get("q"),
		InitiativeID: q.Get("initiative"),
		Quarter:      q.Get("quarter"),
		Year:         yearInt,
	}

	events, err := h.eventRepo.ListAllFiltered(filter)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	initiatives, _ := h.initiativeRepo.ListAll()

	render(w, r, "web/templates/events/list_admin.html", adminEventListData{
		Events:       events,
		StatusFilter: filter.Status,
		Search:       filter.Search,
		InitiativeID: filter.InitiativeID,
		Quarter:      filter.Quarter,
		Year:         filter.Year,
		Initiatives:  initiatives,
	})
}

func (h *AdminEventHandler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
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
		checklistData = BuildChecklistData(e, clItems, true)
	}

	render(w, r, "web/templates/events/show.html", EventShowData{
		Event:     e,
		Comments:  comments,
		Budget:    budget,
		Checklist: checklistData,
	})
}

// AdminEdit renders the event edit form for admins (no status restriction).
func (h *AdminEventHandler) AdminEdit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	initiatives, _ := h.initiativeRepo.ListAll()
	render(w, r, "web/templates/events/edit.html", EventFormData{
		Event:       e,
		Initiatives: initiatives,
	})
}

// AdminUpdate saves admin edits to an event, preserving its current status.
func (h *AdminEventHandler) AdminUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	updated := parseEventForm(r)
	updated.ID = id
	updated.UserID = e.UserID // preserve original submitter

	imagePath, err := h.adminSaveImage(r, e.ImagePath)
	if err != nil {
		setFlash(w, "error", err.Error())
		http.Redirect(w, r, "/admin/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	updated.ImagePath = imagePath

	if strings.TrimSpace(updated.Name) == "" || strings.TrimSpace(updated.Description) == "" {
		setFlash(w, "error", "Event name and description are required.")
		http.Redirect(w, r, "/admin/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	if updated.Quarter == "" {
		setFlash(w, "error", "Quarter is required.")
		http.Redirect(w, r, "/admin/events/"+id+"/edit", http.StatusSeeOther)
		return
	}
	if updated.Year == 0 {
		setFlash(w, "error", "Year is required.")
		http.Redirect(w, r, "/admin/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	if err := h.eventRepo.AdminUpdate(updated); err != nil {
		setFlash(w, "error", "Failed to update event. Please try again.")
		http.Redirect(w, r, "/admin/events/"+id+"/edit", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event updated.")
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

// AdminDelete permanently deletes an event and all its sub-records.
func (h *AdminEventHandler) AdminDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	if err := h.eventRepo.Delete(id); err != nil {
		setFlash(w, "error", "Failed to delete event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Event deleted.")
	http.Redirect(w, r, "/admin/events", http.StatusSeeOther)
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
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.eventRepo.Approve(id, comment); err != nil {
		setFlash(w, "error", "Failed to approve event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	// Record in activity log
	text := "Event approved."
	if comment != "" {
		text = comment
	}
	_ = h.commentRepo.Create(&models.EventComment{
		EventID:  id,
		UserID:   user.ID,
		UserName: user.Name,
		Comment:  text,
		Type:     models.CommentTypeApproval,
	})

	// Auto-initialize checklist defaults for the newly approved event
	conditionFn := func(cond string) bool {
		if cond == "venue_jamatkhana" {
			return e.VenueType == models.VenueTypeInternal
		}
		return false
	}
	_ = h.checklistRepo.InitializeDefaults(id, user.ID, conditionFn)

	// Send email notification to the event submitter
	go h.emailSvc.SendEventNotification(e.UserEmail, e.UserName, e.Name, id, "approved", comment)

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

// RoadmapPDF generates and streams a PDF version of the roadmap.
func (h *AdminEventHandler) RoadmapPDF(w http.ResponseWriter, r *http.Request) {
	events, err := h.eventRepo.ListForCalendar()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	years := groupByYear(events)

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 18)
	pdf.AddPage()

	pageW, _ := pdf.GetPageSize()
	contentW := pageW - 30 // left+right margins

	// ── Header banner ─────────────────────────────────────────────────────────
	pdf.SetFillColor(10, 22, 40) // #0a1628 navy
	pdf.Rect(0, 0, pageW, 28, "F")

	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetXY(15, 7)
	pdf.Cell(contentW, 8, "IPN Southeast Events")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(180, 195, 220)
	pdf.SetXY(15, 17)
	pdf.Cell(contentW, 5, "Event Roadmap - Generated "+time.Now().Format("January 2, 2006"))

	pdf.SetXY(15, 34)

	// quarter colour palette  (R, G, B) pairs
	type qStyle struct {
		label    string
		months   string
		r, g, b  int // header bg
		tr, tg, tb int // header text
		br, bg_, bb int // border
	}
	qStyles := map[string]qStyle{
		"Q1": {"Q1", "January - March", 59, 130, 246, 30, 64, 175, 147, 197, 253},
		"Q2": {"Q2", "April - June", 34, 197, 94, 20, 83, 45, 134, 239, 172},
		"Q3": {"Q3", "July - September", 234, 179, 8, 113, 63, 18, 253, 224, 71},
		"Q4": {"Q4", "October - December", 168, 85, 247, 88, 28, 135, 216, 180, 254},
	}

	// pdfSafe converts a UTF-8 string to Latin-1 safe bytes for fpdf.
	// fpdf uses Latin-1 internally; passing raw UTF-8 causes multi-byte chars
	// (en dash, smart quotes, etc.) to render as mojibake.
	// Steps: (1) replace common Unicode punctuation with ASCII, (2) write
	// U+0000–U+00FF as their raw Latin-1 byte, (3) replace anything above U+00FF with '?'.
	pdfSafe := func(s string) string {
		s = strings.NewReplacer(
			"\u2013", "-",   // en dash
			"\u2014", "-",   // em dash
			"\u2018", "'",   // left single quotation mark
			"\u2019", "'",   // right single quotation mark
			"\u201C", "\"",  // left double quotation mark
			"\u201D", "\"",  // right double quotation mark
			"\u2026", "...", // horizontal ellipsis
			"\u00A0", " ",   // non-breaking space
		).Replace(s)
		var b strings.Builder
		b.Grow(len(s))
		for _, c := range s {
			if c <= 0xFF {
				b.WriteByte(byte(c)) // write the Latin-1 byte value directly
			} else {
				b.WriteByte('?')
			}
		}
		return b.String()
	}

	truncate := func(s string, max int) string {
		if len([]rune(s)) <= max {
			return s
		}
		return string([]rune(s)[:max-1]) + "..."
	}

	for _, cy := range years {
		// ── Year banner ───────────────────────────────────────────────────────
		pdf.Ln(4)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(10, 22, 40)

		yearLabel := fmt.Sprintf("%d", cy.Year)
		if cy.Year == 0 {
			yearLabel = "Unscheduled"
		}

		// Pill background
		tw := pdf.GetStringWidth(yearLabel) + 10
		pdf.SetFillColor(10, 22, 40)
		pdf.RoundedRect(pdf.GetX(), pdf.GetY(), tw, 7, 3, "1234", "F")
		pdf.SetTextColor(255, 255, 255)
		pdf.SetXY(pdf.GetX(), pdf.GetY())
		pdf.CellFormat(tw, 7, yearLabel, "", 0, "C", false, 0, "")

		// Dashed line to the right
		pdf.SetDrawColor(200, 200, 200)
		lineY := pdf.GetY() + 3.5
		lineX := pdf.GetX() + 4
		for x := lineX; x < pageW-15; x += 4 {
			pdf.Line(x, lineY, x+2, lineY)
		}
		pdf.Ln(10)

		// ── Quarters ─────────────────────────────────────────────────────────
		quarters := []struct {
			key    string
			events []*models.Event
		}{
			{"Q1", cy.Q1},
			{"Q2", cy.Q2},
			{"Q3", cy.Q3},
			{"Q4", cy.Q4},
		}
		if len(cy.Unscheduled) > 0 {
			quarters = append(quarters, struct {
				key    string
				events []*models.Event
			}{"No Quarter", cy.Unscheduled})
		}

		for _, q := range quarters {
			if len(q.events) == 0 {
				continue
			}

			qs, hasStyle := qStyles[q.key]
			if !hasStyle {
				qs = qStyle{"-", "", 150, 150, 150, 60, 60, 60, 200, 200, 200}
			}

			// Ensure the header + at least one row fits on the current page
			pdf.SetAutoPageBreak(false, 0)
			if pdf.GetY()+12+float64(len(q.events))*9 > 277 {
				pdf.AddPage()
				pdf.SetXY(15, 20)
			}
			pdf.SetAutoPageBreak(true, 18)

			startX := pdf.GetX()
			startY := pdf.GetY()

			// Quarter header bar
			pdf.SetFillColor(qs.r, qs.g, qs.b)
			pdf.SetDrawColor(qs.br, qs.bg_, qs.bb)
			pdf.RoundedRectExt(startX, startY, contentW, 8, 2, 2, 0, 0, "FD")

			pdf.SetFont("Helvetica", "B", 10)
			pdf.SetTextColor(qs.tr, qs.tg, qs.tb)
			pdf.SetXY(startX+3, startY+1.5)
			pdf.Cell(20, 5, qs.label)

			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(qs.tr, qs.tg, qs.tb)
			pdf.SetXY(startX+22, startY+1.5)
			pdf.Cell(60, 5, qs.months)

			countLabel := fmt.Sprintf("%d event", len(q.events))
			if len(q.events) != 1 {
				countLabel += "s"
			}
			pdf.SetFont("Helvetica", "", 8)
			pdf.SetXY(startX, startY+1.5)
			pdf.CellFormat(contentW-3, 5, countLabel, "", 0, "R", false, 0, "")

			// Event rows
			for i, e := range q.events {
				rowY := startY + 8 + float64(i)*9
				// Zebra stripe
				if i%2 == 0 {
					pdf.SetFillColor(248, 250, 252)
				} else {
					pdf.SetFillColor(255, 255, 255)
				}
				// Round only the bottom corners on the last row (matches rounded top of header).
				rBR, rBL := 0.0, 0.0
				if i == len(q.events)-1 {
					rBR, rBL = 2.0, 2.0
				}
				pdf.SetDrawColor(qs.br, qs.bg_, qs.bb)
				pdf.RoundedRectExt(startX, rowY, contentW, 9, 0, 0, rBR, rBL, "FD")

				// Event name
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetTextColor(30, 30, 30)
				pdf.SetXY(startX+4, rowY+1.5)
				name := truncate(pdfSafe(e.Name), 55)
				pdf.Cell(contentW*0.55, 5, name)

				// Description (truncated)
				if e.Description != "" {
					pdf.SetFont("Helvetica", "", 8)
					pdf.SetTextColor(120, 120, 120)
					desc := truncate(pdfSafe(e.Description), 50)
					pdf.SetXY(startX+4, rowY+5.5)
					pdf.Cell(contentW*0.7, 3.5, desc)
				}

				// Submitter name (right-aligned)
				if e.UserName != "" {
					pdf.SetFont("Helvetica", "", 8)
					pdf.SetTextColor(150, 150, 150)
					pdf.SetXY(startX, rowY+1.5)
					pdf.CellFormat(contentW-3, 5, pdfSafe(e.UserName), "", 0, "R", false, 0, "")
				}
			}

			pdf.SetXY(15, startY+8+float64(len(q.events))*9+4)
		}

		pdf.Ln(4)
	}

	// ── Footer ────────────────────────────────────────────────────────────────
	// Disable auto page break so SetY(-12) doesn't push the footer onto a new page.
	pdf.SetAutoPageBreak(false, 0)
	pdf.SetY(-12)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(160, 160, 160)
	pdf.CellFormat(contentW, 5, "IPN Southeast Events - Confidential", "", 0, "C", false, 0, "")

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `attachment; filename="ipn-roadmap.pdf"`)
	if err := pdf.Output(w); err != nil {
		http.Error(w, "Failed to generate PDF", http.StatusInternalServerError)
	}
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
	user := middleware.UserFromContext(r.Context())

	if comment == "" {
		setFlash(w, "error", "A comment is required when rejecting an event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if err := h.eventRepo.Reject(id, comment); err != nil {
		setFlash(w, "error", "Failed to reject event.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	// Record in activity log
	_ = h.commentRepo.Create(&models.EventComment{
		EventID:  id,
		UserID:   user.ID,
		UserName: user.Name,
		Comment:  comment,
		Type:     models.CommentTypeRejection,
	})

	// Send email notification to the event submitter
	go h.emailSvc.SendEventNotification(e.UserEmail, e.UserName, e.Name, id, "rejected", comment)

	setFlash(w, "success", "Event rejected with feedback.")
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

// AddComment lets an admin post a general comment on any event.
func (h *AdminEventHandler) AddComment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	text := strings.TrimSpace(r.FormValue("comment"))
	user := middleware.UserFromContext(r.Context())

	if text == "" {
		setFlash(w, "error", "Comment cannot be empty.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	_ = h.commentRepo.Create(&models.EventComment{
		EventID:  id,
		UserID:   user.ID,
		UserName: user.Name,
		Comment:  text,
		Type:     models.CommentTypeComment,
	})

	// Send email notification to the event submitter
	if e, err := h.eventRepo.GetByID(id); err == nil {
		go h.emailSvc.SendEventNotification(e.UserEmail, e.UserName, e.Name, id, "comment", text)
	}

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

	endDate := strings.TrimSpace(r.FormValue("recurrence_end_date"))
	if endDate != "" {
		if _, err := time.Parse("2006-01-02", endDate); err != nil {
			setFlash(w, "error", "Invalid recurrence end date format. Use YYYY-MM-DD.")
			http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
			return
		}
	}

	if err := h.eventRepo.SetEventDate(id, date, endDate); err != nil {
		setFlash(w, "error", "Failed to set event date.")
		http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Dates saved.")
	http.Redirect(w, r, "/admin/events/"+id, http.StatusSeeOther)
}

// ExportCSV streams all approved events as a CSV file.
func (h *AdminEventHandler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	events, err := h.eventRepo.ListAll("")
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="ipn_events_%s.csv"`, time.Now().Format("2006-01-02")))

	writer := csv.NewWriter(w)
	_ = writer.Write([]string{
		"Name", "Status", "Submitter", "Quarter", "Year",
		"Event Date", "Start Time", "End Time", "Recurrence",
		"Description", "City", "Scope", "Venue Type",
		"Outcome", "Impact",
		"Financial Resources", "Facilities", "Human Support", "Technology", "Partnerships",
		"Structured Programming", "Engagement Design", "Content Delivery", "Community Building",
	})

	for _, e := range events {
		_ = writer.Write([]string{
			e.Name, e.Status, e.UserName, e.Quarter, strconv.Itoa(e.Year),
			e.EventDate, e.StartTime, e.EndTime, e.Recurrence,
			e.Description, e.City, e.Scope, e.VenueType,
			e.Outcome, e.Impact,
			e.Input.FinancialResources, e.Input.Facilities, e.Input.HumanSupport, e.Input.Technology, e.Input.Partnerships,
			e.Activities.StructuredProgramming, e.Activities.EngagementDesign, e.Activities.ContentDelivery, e.Activities.CommunityBuilding,
		})
	}
	writer.Flush()
}

// DownloadTemplate streams a CSV template file with the required column headers.
func (h *AdminEventHandler) DownloadTemplate(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="events_template.csv"`)

	headers := []string{
		"name", "quarter", "year", "description", "recurrence", "recurrence_end_date",
		"event_date", "start_time", "end_time",
		"outcome", "impact",
		"financial_resources", "facilities", "human_support", "technology", "partnerships",
		"structured_programming", "engagement_design", "content_delivery", "community_building",
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

	for _, col := range []string{"name", "quarter", "year", "description"} {
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
			UserID:            adminUser.ID,
			Name:              getCol(row, "name"),
			Quarter:           getCol(row, "quarter"),
			Year:              yearInt,
			Recurrence:        recurrence,
			RecurrenceEndDate: getCol(row, "recurrence_end_date"),
			EventDate:         getCol(row, "event_date"),
			StartTime:         getCol(row, "start_time"),
			EndTime:           getCol(row, "end_time"),
			Description:       getCol(row, "description"),
			Outcome:           getCol(row, "outcome"),
			Impact:            getCol(row, "impact"),
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

		if strings.TrimSpace(e.Name) == "" || strings.TrimSpace(e.Description) == "" || e.Year == 0 || e.Quarter == "" {
			setFlash(w, "error", fmt.Sprintf("Row %d: name, quarter, description, and year are required.", lineNum))
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
