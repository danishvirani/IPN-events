package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

// Create inserts a new event and all its sub-records in a single transaction.
func (r *EventRepository) Create(e *models.Event) error {
	e.ID = uuid.New().String()
	e.Status = models.StatusPending
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Default assigned_to to creator if not set
	if e.AssignedToID == "" {
		e.AssignedToID = e.UserID
	}

	if _, err := tx.Exec(`
		INSERT INTO events (id, user_id, name, quarter, year, description, recurrence, recurrence_end_date,
		                    event_date, start_time, end_time, image_path,
		                    city, scope, scope_jamatkhana, venue_type, venue_jamatkhana, venue_address,
		                    outcome, impact, status, assigned_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.UserID,
		nullIfEmpty(e.Name), nullIfEmpty(e.Quarter), nullIfZero(e.Year),
		e.Description, e.Recurrence, nullIfEmpty(e.RecurrenceEndDate),
		nullIfEmpty(e.EventDate), nullIfEmpty(e.StartTime), nullIfEmpty(e.EndTime), nullIfEmpty(e.ImagePath),
		nullIfEmpty(e.City), e.Scope, nullIfEmpty(e.ScopeJamatkhana),
		e.VenueType, nullIfEmpty(e.VenueJamatkhana), nullIfEmpty(e.VenueAddress),
		e.Outcome, e.Impact, e.Status, e.AssignedToID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO event_inputs (id, event_id, financial_resources, facilities, human_support, technology, partnerships)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), e.ID,
		e.Input.FinancialResources, e.Input.Facilities, e.Input.HumanSupport,
		e.Input.Technology, e.Input.Partnerships,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		INSERT INTO event_activities (id, event_id, structured_programming, engagement_design, content_delivery, community_building)
		VALUES (?, ?, ?, ?, ?, ?)`,
		uuid.New().String(), e.ID,
		e.Activities.StructuredProgramming, e.Activities.EngagementDesign,
		e.Activities.ContentDelivery, e.Activities.CommunityBuilding,
	); err != nil {
		return err
	}

	for i, oi := range e.OutputItems {
		if _, err := tx.Exec(`
			INSERT INTO event_output_items (id, event_id, description, sort_order)
			VALUES (?, ?, ?, ?)`,
			uuid.New().String(), e.ID, oi.Description, i,
		); err != nil {
			return err
		}
	}

	for i, sr := range e.SupportRequests {
		if _, err := tx.Exec(`
			INSERT INTO event_support_requests (id, event_id, type, description, venue_type, venue_detail, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), e.ID, sr.Type, sr.Description,
			nullIfEmpty(sr.VenueType), nullIfEmpty(sr.VenueDetail), i,
		); err != nil {
			return err
		}
	}

	if err := setEventInitiativesTx(tx, e.ID, e.InitiativeIDs); err != nil {
		return err
	}

	return tx.Commit()
}

// Update replaces all fields of an existing event and resets status to pending.
func (r *EventRepository) Update(e *models.Event) error {
	e.UpdatedAt = time.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE events SET name=?, quarter=?, year=?, description=?, recurrence=?, recurrence_end_date=?,
		                  event_date=?, start_time=?, end_time=?, image_path=?,
		                  city=?, scope=?, scope_jamatkhana=?, venue_type=?, venue_jamatkhana=?, venue_address=?,
		                  outcome=?, impact=?,
		                  status=?, admin_comment=NULL, updated_at=datetime('now')
		WHERE id=?`,
		e.Name, nullIfEmpty(e.Quarter), nullIfZero(e.Year),
		e.Description, e.Recurrence, nullIfEmpty(e.RecurrenceEndDate),
		nullIfEmpty(e.EventDate), nullIfEmpty(e.StartTime), nullIfEmpty(e.EndTime), nullIfEmpty(e.ImagePath),
		nullIfEmpty(e.City), e.Scope, nullIfEmpty(e.ScopeJamatkhana),
		e.VenueType, nullIfEmpty(e.VenueJamatkhana), nullIfEmpty(e.VenueAddress),
		e.Outcome, e.Impact, models.StatusPending, e.ID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE event_inputs SET financial_resources=?, facilities=?, human_support=?, technology=?, partnerships=?
		WHERE event_id=?`,
		e.Input.FinancialResources, e.Input.Facilities, e.Input.HumanSupport,
		e.Input.Technology, e.Input.Partnerships, e.ID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE event_activities SET structured_programming=?, engagement_design=?, content_delivery=?, community_building=?
		WHERE event_id=?`,
		e.Activities.StructuredProgramming, e.Activities.EngagementDesign,
		e.Activities.ContentDelivery, e.Activities.CommunityBuilding, e.ID,
	); err != nil {
		return err
	}

	// Replace output items
	if _, err := tx.Exec(`DELETE FROM event_output_items WHERE event_id=?`, e.ID); err != nil {
		return err
	}
	for i, oi := range e.OutputItems {
		if _, err := tx.Exec(`
			INSERT INTO event_output_items (id, event_id, description, sort_order)
			VALUES (?, ?, ?, ?)`,
			uuid.New().String(), e.ID, oi.Description, i,
		); err != nil {
			return err
		}
	}

	// Replace support requests
	if _, err := tx.Exec(`DELETE FROM event_support_requests WHERE event_id=?`, e.ID); err != nil {
		return err
	}
	for i, sr := range e.SupportRequests {
		if _, err := tx.Exec(`
			INSERT INTO event_support_requests (id, event_id, type, description, venue_type, venue_detail, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), e.ID, sr.Type, sr.Description,
			nullIfEmpty(sr.VenueType), nullIfEmpty(sr.VenueDetail), i,
		); err != nil {
			return err
		}
	}

	if err := setEventInitiativesTx(tx, e.ID, e.InitiativeIDs); err != nil {
		return err
	}

	return tx.Commit()
}

// AdminUpdate replaces all fields of an existing event while preserving its current status.
func (r *EventRepository) AdminUpdate(e *models.Event) error {
	e.UpdatedAt = time.Now()

	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`
		UPDATE events SET name=?, quarter=?, year=?, description=?, recurrence=?, recurrence_end_date=?,
		                  event_date=?, start_time=?, end_time=?, image_path=?,
		                  city=?, scope=?, scope_jamatkhana=?, venue_type=?, venue_jamatkhana=?, venue_address=?,
		                  outcome=?, impact=?, updated_at=datetime('now')
		WHERE id=?`,
		e.Name, nullIfEmpty(e.Quarter), nullIfZero(e.Year),
		e.Description, e.Recurrence, nullIfEmpty(e.RecurrenceEndDate),
		nullIfEmpty(e.EventDate), nullIfEmpty(e.StartTime), nullIfEmpty(e.EndTime), nullIfEmpty(e.ImagePath),
		nullIfEmpty(e.City), e.Scope, nullIfEmpty(e.ScopeJamatkhana),
		e.VenueType, nullIfEmpty(e.VenueJamatkhana), nullIfEmpty(e.VenueAddress),
		e.Outcome, e.Impact, e.ID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE event_inputs SET financial_resources=?, facilities=?, human_support=?, technology=?, partnerships=?
		WHERE event_id=?`,
		e.Input.FinancialResources, e.Input.Facilities, e.Input.HumanSupport,
		e.Input.Technology, e.Input.Partnerships, e.ID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`
		UPDATE event_activities SET structured_programming=?, engagement_design=?, content_delivery=?, community_building=?
		WHERE event_id=?`,
		e.Activities.StructuredProgramming, e.Activities.EngagementDesign,
		e.Activities.ContentDelivery, e.Activities.CommunityBuilding, e.ID,
	); err != nil {
		return err
	}

	if _, err := tx.Exec(`DELETE FROM event_output_items WHERE event_id=?`, e.ID); err != nil {
		return err
	}
	for i, oi := range e.OutputItems {
		if _, err := tx.Exec(`
			INSERT INTO event_output_items (id, event_id, description, sort_order)
			VALUES (?, ?, ?, ?)`,
			uuid.New().String(), e.ID, oi.Description, i,
		); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`DELETE FROM event_support_requests WHERE event_id=?`, e.ID); err != nil {
		return err
	}
	for i, sr := range e.SupportRequests {
		if _, err := tx.Exec(`
			INSERT INTO event_support_requests (id, event_id, type, description, venue_type, venue_detail, sort_order)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), e.ID, sr.Type, sr.Description,
			nullIfEmpty(sr.VenueType), nullIfEmpty(sr.VenueDetail), i,
		); err != nil {
			return err
		}
	}

	if err := setEventInitiativesTx(tx, e.ID, e.InitiativeIDs); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete removes an event and all its sub-records in a single transaction.
func (r *EventRepository) Delete(id string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, table := range []string{
		"event_checklist_items",
		"event_budget_items",
		"event_initiatives",
		"event_comments",
		"event_support_requests",
		"event_output_items",
		"event_activities",
		"event_inputs",
	} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE event_id=?`, id); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`DELETE FROM events WHERE id=?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// GetByID fetches a full event with all sub-records.
func (r *EventRepository) GetByID(id string) (*models.Event, error) {
	e := &models.Event{}
	var quarter, outcome, impact, adminComment, recurrence, recurrenceEndDate, eventDate sql.NullString
	var startTime, endTime, imagePath sql.NullString
	var city, scopeJK, venueJK, venueAddr sql.NullString
	var assignedTo, assignedToName sql.NullString
	var year sql.NullInt64
	var registrationCount, participationCount, attendanceCount sql.NullInt64
	var registrationMode sql.NullString

	err := r.db.QueryRow(`
		SELECT e.id, e.user_id, u.name, u.email,
		       e.name, e.quarter, e.year, e.description, e.recurrence, e.recurrence_end_date, e.event_date,
		       e.start_time, e.end_time, e.image_path,
		       e.city, e.scope, e.scope_jamatkhana, e.venue_type, e.venue_jamatkhana, e.venue_address,
		       e.outcome, e.impact, e.status, e.admin_comment, e.created_at, e.updated_at,
		       COALESCE(e.assigned_to, e.user_id), COALESCE(a.name, u.name),
		       e.registration_count, e.participation_count,
		       e.is_paid_event,
		       e.registration_mode, e.attendance_count,
		       e.completed,
		       e.output_achievement, e.outcome_achievement, e.impact_achievement,
		       e.swot_strengths, e.swot_weaknesses, e.swot_opportunities, e.swot_threats
		FROM events e
		JOIN users u ON e.user_id = u.id
		LEFT JOIN users a ON e.assigned_to = a.id
		WHERE e.id = ?`, id,
	).Scan(
		&e.ID, &e.UserID, &e.UserName, &e.UserEmail,
		&e.Name, &quarter, &year, &e.Description, &recurrence, &recurrenceEndDate, &eventDate,
		&startTime, &endTime, &imagePath,
		&city, &e.Scope, &scopeJK, &e.VenueType, &venueJK, &venueAddr,
		&outcome, &impact, &e.Status, &adminComment, &e.CreatedAt, &e.UpdatedAt,
		&assignedTo, &assignedToName,
		&registrationCount, &participationCount,
		&e.IsPaidEvent,
		&registrationMode, &attendanceCount,
		&e.Completed,
		&e.OutputAchievement, &e.OutcomeAchievement, &e.ImpactAchievement,
		&e.SWOTStrengths, &e.SWOTWeaknesses, &e.SWOTOpportunities, &e.SWOTThreats,
	)
	if err != nil {
		return nil, err
	}
	e.AssignedToID = assignedTo.String
	e.AssignedToName = assignedToName.String
	e.Quarter = quarter.String
	e.Year = int(year.Int64)
	e.Recurrence = recurrence.String
	if e.Recurrence == "" {
		e.Recurrence = models.RecurrenceNone
	}
	e.RecurrenceEndDate = recurrenceEndDate.String
	e.EventDate = eventDate.String
	e.StartTime = startTime.String
	e.EndTime = endTime.String
	e.ImagePath = imagePath.String
	e.City = city.String
	e.ScopeJamatkhana = scopeJK.String
	e.VenueJamatkhana = venueJK.String
	e.VenueAddress = venueAddr.String
	e.Outcome = outcome.String
	e.Impact = impact.String
	e.AdminComment = adminComment.String
	e.RegistrationCount = int(registrationCount.Int64)
	e.ParticipationCount = int(participationCount.Int64)
	e.RegistrationMode = registrationMode.String
	if e.RegistrationMode == "" {
		e.RegistrationMode = "full"
	}
	e.AttendanceCount = int(attendanceCount.Int64)

	// Sub-tables
	var fi, ff, fh, ft, fp sql.NullString
	_ = r.db.QueryRow(`
		SELECT financial_resources, facilities, human_support, technology, partnerships
		FROM event_inputs WHERE event_id=?`, e.ID,
	).Scan(&fi, &ff, &fh, &ft, &fp)
	e.Input = models.EventInput{
		FinancialResources: fi.String, Facilities: ff.String,
		HumanSupport: fh.String, Technology: ft.String, Partnerships: fp.String,
	}

	var as, ae, ac, ab sql.NullString
	_ = r.db.QueryRow(`
		SELECT structured_programming, engagement_design, content_delivery, community_building
		FROM event_activities WHERE event_id=?`, e.ID,
	).Scan(&as, &ae, &ac, &ab)
	e.Activities = models.EventActivities{
		StructuredProgramming: as.String, EngagementDesign: ae.String,
		ContentDelivery: ac.String, CommunityBuilding: ab.String,
	}

	outputRows, err := r.db.Query(`
		SELECT id, event_id, description, sort_order
		FROM event_output_items WHERE event_id=? ORDER BY sort_order`, e.ID,
	)
	if err == nil {
		defer outputRows.Close()
		for outputRows.Next() {
			var oi models.OutputItem
			if err := outputRows.Scan(&oi.ID, &oi.EventID, &oi.Description, &oi.SortOrder); err == nil {
				e.OutputItems = append(e.OutputItems, oi)
			}
		}
	}

	srRows, err := r.db.Query(`
		SELECT id, event_id, type, description, COALESCE(venue_type,''), COALESCE(venue_detail,''), sort_order
		FROM event_support_requests WHERE event_id=? ORDER BY sort_order`, e.ID,
	)
	if err == nil {
		defer srRows.Close()
		for srRows.Next() {
			var sr models.SupportRequest
			if err := srRows.Scan(&sr.ID, &sr.EventID, &sr.Type, &sr.Description, &sr.VenueType, &sr.VenueDetail, &sr.SortOrder); err == nil {
				e.SupportRequests = append(e.SupportRequests, sr)
			}
		}
	}

	// Load tagged initiatives
	initRows, err := r.db.Query(`
		SELECT si.id, si.name, si.objective, si.created_at, si.updated_at
		FROM strategic_initiatives si
		JOIN event_initiatives ei ON si.id = ei.initiative_id
		WHERE ei.event_id = ?
		ORDER BY si.name`, e.ID,
	)
	if err == nil {
		defer initRows.Close()
		for initRows.Next() {
			var init models.Initiative
			if err := initRows.Scan(&init.ID, &init.Name, &init.Objective, &init.CreatedAt, &init.UpdatedAt); err == nil {
				e.Initiatives = append(e.Initiatives, init)
			}
		}
	}

	return e, nil
}

// ListByUser returns all events submitted by or assigned to a user, newest first.
func (r *EventRepository) ListByUser(userID string) ([]*models.Event, error) {
	return r.listEvents(`WHERE (e.user_id = ? OR e.assigned_to = ?) ORDER BY e.created_at DESC`, userID, userID)
}

// ListAll returns all events for admin view, newest first.
func (r *EventRepository) ListAll(statusFilter string) ([]*models.Event, error) {
	if statusFilter != "" {
		return r.listEvents(`WHERE e.status = ? ORDER BY e.created_at DESC`, statusFilter)
	}
	return r.listEvents(`ORDER BY e.created_at DESC`)
}

// EventFilter holds search and filter parameters for the admin event list.
type EventFilter struct {
	Status       string
	Search       string // search by event name (LIKE)
	InitiativeID string // filter by initiative
	Quarter      string // e.g. "Q1"
	Year         int    // e.g. 2025
}

// ListAllFiltered returns events matching the given filters.
func (r *EventRepository) ListAllFiltered(f EventFilter) ([]*models.Event, error) {
	var conditions []string
	var args []any

	if f.Status == "completed" {
		conditions = append(conditions, "e.completed = 1")
	} else if f.Status != "" {
		conditions = append(conditions, "e.status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		conditions = append(conditions, "e.name LIKE ?")
		args = append(args, "%"+f.Search+"%")
	}
	if f.InitiativeID != "" {
		conditions = append(conditions, "e.id IN (SELECT event_id FROM event_initiatives WHERE initiative_id = ?)")
		args = append(args, f.InitiativeID)
	}
	if f.Quarter != "" {
		conditions = append(conditions, "e.quarter = ?")
		args = append(args, f.Quarter)
	}
	if f.Year > 0 {
		conditions = append(conditions, "e.year = ?")
		args = append(args, f.Year)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	return r.listEvents(where+" ORDER BY e.created_at DESC", args...)
}

// ListForCalendar returns all approved events ordered by year then quarter.
func (r *EventRepository) ListForCalendar() ([]*models.Event, error) {
	return r.listEvents(`WHERE e.status = ? ORDER BY COALESCE(e.year, 9999) ASC, COALESCE(e.quarter, 'Z') ASC, e.name ASC`, models.StatusApproved)
}

func (r *EventRepository) listEvents(whereClause string, args ...any) ([]*models.Event, error) {
	query := `
		SELECT e.id, e.user_id, u.name, u.email,
		       e.name, e.quarter, e.year, e.description,
		       e.recurrence, e.recurrence_end_date, e.event_date,
		       e.start_time, e.end_time, e.image_path,
		       e.city, e.scope,
		       e.status, e.admin_comment, e.created_at, e.updated_at,
		       e.completed
		FROM events e
		JOIN users u ON e.user_id = u.id ` + whereClause

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*models.Event
	for rows.Next() {
		e := &models.Event{}
		var quarter, adminComment, recurrence, recurrenceEndDate, eventDate sql.NullString
		var startTime, endTime, imagePath, city, scope sql.NullString
		var year sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.UserName, &e.UserEmail,
			&e.Name, &quarter, &year, &e.Description,
			&recurrence, &recurrenceEndDate, &eventDate,
			&startTime, &endTime, &imagePath,
			&city, &scope,
			&e.Status, &adminComment, &e.CreatedAt, &e.UpdatedAt,
			&e.Completed,
		); err != nil {
			return nil, err
		}
		e.Quarter = quarter.String
		e.Year = int(year.Int64)
		e.Recurrence = recurrence.String
		if e.Recurrence == "" {
			e.Recurrence = models.RecurrenceNone
		}
		e.RecurrenceEndDate = recurrenceEndDate.String
		e.EventDate = eventDate.String
		e.StartTime = startTime.String
		e.EndTime = endTime.String
		e.ImagePath = imagePath.String
		e.City = city.String
		e.Scope = scope.String
		e.AdminComment = adminComment.String
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load initiatives for all events in a single batch
	if len(events) > 0 {
		r.loadInitiativesForEvents(events)
	}

	return events, nil
}

// loadInitiativesForEvents populates the Initiatives field for a list of events.
func (r *EventRepository) loadInitiativesForEvents(events []*models.Event) {
	// Build event ID index
	idx := make(map[string]*models.Event, len(events))
	for _, e := range events {
		idx[e.ID] = e
	}

	// Query all initiative links for these events
	placeholders := make([]string, len(events))
	args := make([]any, len(events))
	for i, e := range events {
		placeholders[i] = "?"
		args[i] = e.ID
	}
	query := `
		SELECT ei.event_id, si.id, si.name, si.objective, si.created_at, si.updated_at
		FROM event_initiatives ei
		JOIN strategic_initiatives si ON si.id = ei.initiative_id
		WHERE ei.event_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY si.name`
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var eventID string
		var init models.Initiative
		if err := rows.Scan(&eventID, &init.ID, &init.Name, &init.Objective, &init.CreatedAt, &init.UpdatedAt); err == nil {
			if e, ok := idx[eventID]; ok {
				e.Initiatives = append(e.Initiatives, init)
			}
		}
	}
}

// Approve sets the event status to approved.
func (r *EventRepository) Approve(id, comment string) error {
	_, err := r.db.Exec(`
		UPDATE events SET status=?, admin_comment=?, updated_at=datetime('now') WHERE id=?`,
		models.StatusApproved, comment, id,
	)
	return err
}

// UpdateAssignedTo changes who the event is assigned to.
func (r *EventRepository) UpdateAssignedTo(eventID, userID string) error {
	_, err := r.db.Exec(`UPDATE events SET assigned_to=?, updated_at=datetime('now') WHERE id=?`, userID, eventID)
	return err
}

// Reject sets the event status to rejected with a required comment.
func (r *EventRepository) Reject(id, comment string) error {
	_, err := r.db.Exec(`
		UPDATE events SET status=?, admin_comment=?, updated_at=datetime('now') WHERE id=?`,
		models.StatusRejected, comment, id,
	)
	return err
}

// CountByStatus returns counts for a given user (or all users if userID is empty).
func (r *EventRepository) CountByStatus(userID string) (pending, approved, rejected int, err error) {
	query := `SELECT status, COUNT(*) FROM events`
	var args []any
	if userID != "" {
		query += ` WHERE (user_id = ? OR assigned_to = ?)`
		args = append(args, userID, userID)
	}
	query += ` GROUP BY status`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int
		if err = rows.Scan(&status, &count); err != nil {
			return
		}
		switch status {
		case models.StatusPending:
			pending = count
		case models.StatusApproved:
			approved = count
		case models.StatusRejected:
			rejected = count
		}
	}
	return
}

// SetEventDate sets or clears the event_date and recurrence_end_date for an approved event.
func (r *EventRepository) SetEventDate(id, date, recurrenceEndDate string) error {
	_, err := r.db.Exec(
		`UPDATE events SET event_date=?, recurrence_end_date=?, updated_at=datetime('now') WHERE id=?`,
		nullIfEmpty(date), nullIfEmpty(recurrenceEndDate), id,
	)
	return err
}

// BulkCreate inserts multiple events in a single transaction, all marked as approved.
func (r *EventRepository) BulkCreate(events []*models.Event) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	now := time.Now()
	for _, e := range events {
		e.ID = uuid.New().String()
		e.Status = models.StatusApproved
		e.CreatedAt = now
		e.UpdatedAt = now

		scope := e.Scope
		if scope == "" {
			scope = models.ScopeRegional
		}
		venueType := e.VenueType
		if venueType == "" {
			venueType = models.VenueTypeExternal
		}

		if _, err := tx.Exec(`
			INSERT INTO events (id, user_id, name, quarter, year, description, recurrence, recurrence_end_date,
			                    start_time, end_time,
			                    city, scope, scope_jamatkhana, venue_type, venue_jamatkhana, venue_address,
			                    outcome, impact, event_date, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.UserID,
			nullIfEmpty(e.Name), nullIfEmpty(e.Quarter), nullIfZero(e.Year),
			e.Description, e.Recurrence, nullIfEmpty(e.RecurrenceEndDate),
			nullIfEmpty(e.StartTime), nullIfEmpty(e.EndTime),
			nullIfEmpty(e.City), scope, nullIfEmpty(e.ScopeJamatkhana),
			venueType, nullIfEmpty(e.VenueJamatkhana), nullIfEmpty(e.VenueAddress),
			e.Outcome, e.Impact, nullIfEmpty(e.EventDate), e.Status,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO event_inputs (id, event_id, financial_resources, facilities, human_support, technology, partnerships)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), e.ID,
			e.Input.FinancialResources, e.Input.Facilities, e.Input.HumanSupport,
			e.Input.Technology, e.Input.Partnerships,
		); err != nil {
			return err
		}

		if _, err := tx.Exec(`
			INSERT INTO event_activities (id, event_id, structured_programming, engagement_design, content_delivery, community_building)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.New().String(), e.ID,
			e.Activities.StructuredProgramming, e.Activities.EngagementDesign,
			e.Activities.ContentDelivery, e.Activities.CommunityBuilding,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// UpdateAttendance sets the registration and participation counts for an approved event.
func (r *EventRepository) UpdateAttendance(id string, registration, participation int) error {
	_, err := r.db.Exec(`
		UPDATE events SET registration_count=?, participation_count=?, updated_at=datetime('now')
		WHERE id=?`,
		nullIfZero(registration), nullIfZero(participation), id,
	)
	return err
}

// DashboardStats returns aggregate event counts for the admin dashboard.
func (r *EventRepository) DashboardStats() (*models.DashboardStats, error) {
	s := &models.DashboardStats{}
	err := r.db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN status='pending' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='approved' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status='rejected' THEN 1 ELSE 0 END),
			COALESCE(SUM(CASE WHEN status='approved' THEN
				COALESCE(NULLIF(registration_count, 0), (SELECT COUNT(*) FROM event_participants WHERE event_id=events.id))
			ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN status='approved' THEN
				COALESCE(NULLIF(attendance_count, 0), NULLIF(participation_count, 0), (SELECT COUNT(*) FROM event_participants WHERE event_id=events.id AND checked_in=1))
			ELSE 0 END), 0)
		FROM events`,
	).Scan(&s.Total, &s.Pending, &s.Approved, &s.Rejected, &s.TotalRegistrations, &s.TotalParticipants)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// RecentEvents returns the N most recently created events.
func (r *EventRepository) RecentEvents(limit int) ([]*models.Event, error) {
	return r.listEvents(`ORDER BY e.created_at DESC LIMIT ?`, limit)
}

// UpcomingEvents returns the next N approved events with a future event_date.
func (r *EventRepository) UpcomingEvents(limit int) ([]*models.Event, error) {
	return r.listEvents(
		`WHERE e.status = ? AND e.event_date IS NOT NULL AND e.event_date >= date('now') ORDER BY e.event_date ASC LIMIT ?`,
		models.StatusApproved, limit,
	)
}

// CountByQuarter returns event counts per quarter for a given year (approved only).
func (r *EventRepository) CountByQuarter(year int) ([]models.QuarterCount, error) {
	rows, err := r.db.Query(`
		SELECT COALESCE(quarter, 'Unset'), COUNT(*)
		FROM events
		WHERE year = ? AND status = 'approved'
		GROUP BY quarter
		ORDER BY quarter`, year,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.QuarterCount
	for rows.Next() {
		var qc models.QuarterCount
		if err := rows.Scan(&qc.Quarter, &qc.Count); err != nil {
			return nil, err
		}
		result = append(result, qc)
	}
	return result, rows.Err()
}

// InitiativeEventCounts returns each initiative with how many events are tagged.
func (r *EventRepository) InitiativeEventCounts() ([]models.InitiativeCount, error) {
	rows, err := r.db.Query(`
		SELECT si.id, si.name, COUNT(ei.event_id)
		FROM strategic_initiatives si
		LEFT JOIN event_initiatives ei ON si.id = ei.initiative_id
		GROUP BY si.id
		ORDER BY COUNT(ei.event_id) DESC, si.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.InitiativeCount
	for rows.Next() {
		var ic models.InitiativeCount
		if err := rows.Scan(&ic.ID, &ic.Name, &ic.Count); err != nil {
			return nil, err
		}
		result = append(result, ic)
	}
	return result, rows.Err()
}

// UpdateRegistrationMode sets the registration_mode for an event.
func (r *EventRepository) UpdateRegistrationMode(id, mode string) error {
	_, err := r.db.Exec(`UPDATE events SET registration_mode=?, updated_at=datetime('now') WHERE id=?`, mode, id)
	return err
}

// ToggleCompleted marks or unmarks an event as completed.
func (r *EventRepository) ToggleCompleted(id string, completed bool) error {
	val := 0
	if completed {
		val = 1
	}
	_, err := r.db.Exec(`UPDATE events SET completed=?, updated_at=datetime('now') WHERE id=?`, val, id)
	return err
}

// UpdateAttendanceCount sets the manual attendance count for an event.
func (r *EventRepository) UpdateAttendanceCount(id string, count int) error {
	_, err := r.db.Exec(`UPDATE events SET attendance_count=?, updated_at=datetime('now') WHERE id=?`, nullIfZero(count), id)
	return err
}

// UpdateIsPaidEvent toggles the is_paid_event flag on an event.
func (r *EventRepository) UpdateIsPaidEvent(id string, isPaid bool) error {
	v := 0
	if isPaid {
		v = 1
	}
	_, err := r.db.Exec(`UPDATE events SET is_paid_event=?, updated_at=datetime('now') WHERE id=?`, v, id)
	return err
}

// UpdateAchievements saves the output/outcome/impact achievement percentages.
func (r *EventRepository) UpdateAchievements(id string, output, outcome, impact int) error {
	_, err := r.db.Exec(`UPDATE events SET output_achievement=?, outcome_achievement=?, impact_achievement=?, updated_at=datetime('now') WHERE id=?`,
		output, outcome, impact, id)
	return err
}

// UpdateSWOT saves the SWOT analysis fields.
func (r *EventRepository) UpdateSWOT(id string, strengths, weaknesses, opportunities, threats string) error {
	_, err := r.db.Exec(`UPDATE events SET swot_strengths=?, swot_weaknesses=?, swot_opportunities=?, swot_threats=?, updated_at=datetime('now') WHERE id=?`,
		strengths, weaknesses, opportunities, threats, id)
	return err
}

// helpers
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func nullIfZero(n int) any {
	if n == 0 {
		return nil
	}
	return n
}

// setEventInitiativesTx replaces the initiative tags for an event within an existing transaction.
func setEventInitiativesTx(tx *sql.Tx, eventID string, initiativeIDs []string) error {
	if _, err := tx.Exec(`DELETE FROM event_initiatives WHERE event_id = ?`, eventID); err != nil {
		return err
	}
	for _, initID := range initiativeIDs {
		if initID == "" {
			continue
		}
		if _, err := tx.Exec(`
			INSERT INTO event_initiatives (event_id, initiative_id) VALUES (?, ?)`,
			eventID, initID,
		); err != nil {
			return err
		}
	}
	return nil
}
