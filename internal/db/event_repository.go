package db

import (
	"database/sql"
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

	if _, err := tx.Exec(`
		INSERT INTO events (id, user_id, name, quarter, year, description, recurrence,
		                    city, scope, scope_jamatkhana, venue_type, venue_jamatkhana, venue_address,
		                    outcome, impact, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.UserID,
		nullIfEmpty(e.Name), nullIfEmpty(e.Quarter), nullIfZero(e.Year),
		e.Description, e.Recurrence,
		nullIfEmpty(e.City), e.Scope, nullIfEmpty(e.ScopeJamatkhana),
		e.VenueType, nullIfEmpty(e.VenueJamatkhana), nullIfEmpty(e.VenueAddress),
		e.Outcome, e.Impact, e.Status,
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
		UPDATE events SET name=?, quarter=?, year=?, description=?, recurrence=?,
		                  city=?, scope=?, scope_jamatkhana=?, venue_type=?, venue_jamatkhana=?, venue_address=?,
		                  outcome=?, impact=?,
		                  status=?, admin_comment=NULL, updated_at=datetime('now')
		WHERE id=?`,
		e.Name, nullIfEmpty(e.Quarter), nullIfZero(e.Year),
		e.Description, e.Recurrence,
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

	return tx.Commit()
}

// GetByID fetches a full event with all sub-records.
func (r *EventRepository) GetByID(id string) (*models.Event, error) {
	e := &models.Event{}
	var quarter, outcome, impact, adminComment, recurrence, eventDate sql.NullString
	var city, scopeJK, venueJK, venueAddr sql.NullString
	var year sql.NullInt64

	err := r.db.QueryRow(`
		SELECT e.id, e.user_id, u.name, u.email,
		       e.name, e.quarter, e.year, e.description, e.recurrence, e.event_date,
		       e.city, e.scope, e.scope_jamatkhana, e.venue_type, e.venue_jamatkhana, e.venue_address,
		       e.outcome, e.impact, e.status, e.admin_comment, e.created_at, e.updated_at
		FROM events e
		JOIN users u ON e.user_id = u.id
		WHERE e.id = ?`, id,
	).Scan(
		&e.ID, &e.UserID, &e.UserName, &e.UserEmail,
		&e.Name, &quarter, &year, &e.Description, &recurrence, &eventDate,
		&city, &e.Scope, &scopeJK, &e.VenueType, &venueJK, &venueAddr,
		&outcome, &impact, &e.Status, &adminComment, &e.CreatedAt, &e.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	e.Quarter = quarter.String
	e.Year = int(year.Int64)
	e.Recurrence = recurrence.String
	if e.Recurrence == "" {
		e.Recurrence = models.RecurrenceNone
	}
	e.EventDate = eventDate.String
	e.City = city.String
	e.ScopeJamatkhana = scopeJK.String
	e.VenueJamatkhana = venueJK.String
	e.VenueAddress = venueAddr.String
	e.Outcome = outcome.String
	e.Impact = impact.String
	e.AdminComment = adminComment.String

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

	return e, nil
}

// ListByUser returns all events for a specific user, newest first.
func (r *EventRepository) ListByUser(userID string) ([]*models.Event, error) {
	return r.listEvents(`WHERE e.user_id = ? ORDER BY e.created_at DESC`, userID)
}

// ListAll returns all events for admin view, newest first.
func (r *EventRepository) ListAll(statusFilter string) ([]*models.Event, error) {
	if statusFilter != "" {
		return r.listEvents(`WHERE e.status = ? ORDER BY e.created_at DESC`, statusFilter)
	}
	return r.listEvents(`ORDER BY e.created_at DESC`)
}

// ListForCalendar returns all approved events ordered by year then quarter.
func (r *EventRepository) ListForCalendar() ([]*models.Event, error) {
	return r.listEvents(`WHERE e.status = ? ORDER BY COALESCE(e.year, 9999) ASC, COALESCE(e.quarter, 'Z') ASC, e.name ASC`, models.StatusApproved)
}

func (r *EventRepository) listEvents(whereClause string, args ...any) ([]*models.Event, error) {
	query := `
		SELECT e.id, e.user_id, u.name, u.email,
		       e.name, e.quarter, e.year, e.description,
		       e.recurrence, e.event_date, e.city, e.scope,
		       e.status, e.admin_comment, e.created_at, e.updated_at
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
		var quarter, adminComment, recurrence, eventDate, city, scope sql.NullString
		var year sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.UserName, &e.UserEmail,
			&e.Name, &quarter, &year, &e.Description,
			&recurrence, &eventDate, &city, &scope,
			&e.Status, &adminComment, &e.CreatedAt, &e.UpdatedAt,
		); err != nil {
			return nil, err
		}
		e.Quarter = quarter.String
		e.Year = int(year.Int64)
		e.Recurrence = recurrence.String
		if e.Recurrence == "" {
			e.Recurrence = models.RecurrenceNone
		}
		e.EventDate = eventDate.String
		e.City = city.String
		e.Scope = scope.String
		e.AdminComment = adminComment.String
		events = append(events, e)
	}
	return events, rows.Err()
}

// Approve sets the event status to approved.
func (r *EventRepository) Approve(id, comment string) error {
	_, err := r.db.Exec(`
		UPDATE events SET status=?, admin_comment=?, updated_at=datetime('now') WHERE id=?`,
		models.StatusApproved, comment, id,
	)
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
		query += ` WHERE user_id = ?`
		args = append(args, userID)
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

// SetEventDate sets or clears the specific event date for an approved event.
func (r *EventRepository) SetEventDate(id, date string) error {
	_, err := r.db.Exec(
		`UPDATE events SET event_date=?, updated_at=datetime('now') WHERE id=?`,
		nullIfEmpty(date), id,
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
			INSERT INTO events (id, user_id, name, quarter, year, description, recurrence,
			                    city, scope, scope_jamatkhana, venue_type, venue_jamatkhana, venue_address,
			                    outcome, impact, event_date, status)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			e.ID, e.UserID,
			nullIfEmpty(e.Name), nullIfEmpty(e.Quarter), nullIfZero(e.Year),
			e.Description, e.Recurrence,
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
