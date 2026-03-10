package db

import (
	"database/sql"
	"strings"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

// ParticipantRepository handles CRUD for event participants.
type ParticipantRepository struct {
	db *sql.DB
}

// NewParticipantRepository creates a new ParticipantRepository.
func NewParticipantRepository(db *sql.DB) *ParticipantRepository {
	return &ParticipantRepository{db: db}
}

// Create inserts a single participant.
func (r *ParticipantRepository) Create(p *models.Participant) error {
	p.ID = uuid.New().String()
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now

	_, err := r.db.Exec(`
		INSERT INTO event_participants (id, event_id, first_name, last_name, email, phone,
		                                jamatkhana, gender, company, role, is_walkin,
		                                checked_in, checked_in_at, paid, paid_at,
		                                created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.EventID, p.FirstName, p.LastName, p.Email, p.Phone,
		p.Jamatkhana, p.Gender, p.Company, p.Role, boolToInt(p.IsWalkin),
		boolToInt(p.CheckedIn), p.CheckedInAt, boolToInt(p.Paid), p.PaidAt,
		p.CreatedAt, p.UpdatedAt,
	)
	return err
}

// BulkUpsert inserts or updates participants in a transaction, matching on (event_id, email).
// Participants without an email are always inserted as new records.
// Returns (created, updated) counts.
func (r *ParticipantRepository) BulkUpsert(eventID string, participants []*models.Participant) (created, updated int, err error) {
	tx, err := r.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	now := time.Now()

	for _, p := range participants {
		p.EventID = eventID

		if p.Email != "" {
			// Check if participant with this email already exists for this event
			var existingID string
			err := tx.QueryRow(
				`SELECT id FROM event_participants WHERE event_id = ? AND email = ?`,
				eventID, p.Email,
			).Scan(&existingID)

			if err == nil {
				// Update existing — preserve check-in/paid state
				_, err = tx.Exec(`
					UPDATE event_participants
					SET first_name=?, last_name=?, phone=?, jamatkhana=?, gender=?,
					    company=?, role=?, updated_at=?
					WHERE id=?`,
					p.FirstName, p.LastName, p.Phone, p.Jamatkhana, p.Gender,
					p.Company, p.Role, now, existingID,
				)
				if err != nil {
					return 0, 0, err
				}
				updated++
				continue
			}
		}

		// Insert new participant
		id := uuid.New().String()
		_, err = tx.Exec(`
			INSERT INTO event_participants (id, event_id, first_name, last_name, email, phone,
			                                jamatkhana, gender, company, role, is_walkin,
			                                created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
			id, eventID, p.FirstName, p.LastName, p.Email, p.Phone,
			p.Jamatkhana, p.Gender, p.Company, p.Role, now, now,
		)
		if err != nil {
			return 0, 0, err
		}
		created++
	}

	return created, updated, tx.Commit()
}

// GetByID fetches a single participant.
func (r *ParticipantRepository) GetByID(id string) (*models.Participant, error) {
	p := &models.Participant{}
	var isWalkin, checkedIn, paid int
	err := r.db.QueryRow(`
		SELECT id, event_id, first_name, last_name, email, phone,
		       jamatkhana, gender, company, role, is_walkin,
		       checked_in, checked_in_at, paid, paid_at,
		       created_at, updated_at
		FROM event_participants WHERE id = ?`, id,
	).Scan(
		&p.ID, &p.EventID, &p.FirstName, &p.LastName, &p.Email, &p.Phone,
		&p.Jamatkhana, &p.Gender, &p.Company, &p.Role, &isWalkin,
		&checkedIn, &p.CheckedInAt, &paid, &p.PaidAt,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	p.IsWalkin = isWalkin == 1
	p.CheckedIn = checkedIn == 1
	p.Paid = paid == 1
	return p, nil
}

// ListByEvent returns all participants for an event, ordered by last_name, first_name.
func (r *ParticipantRepository) ListByEvent(eventID string) ([]*models.Participant, error) {
	rows, err := r.db.Query(`
		SELECT id, event_id, first_name, last_name, email, phone,
		       jamatkhana, gender, company, role, is_walkin,
		       checked_in, checked_in_at, paid, paid_at,
		       created_at, updated_at
		FROM event_participants
		WHERE event_id = ?
		ORDER BY last_name ASC, first_name ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParticipants(rows)
}

// SearchByEvent returns participants matching a search query across all fields.
func (r *ParticipantRepository) SearchByEvent(eventID, query string) ([]*models.Participant, error) {
	like := "%" + query + "%"
	rows, err := r.db.Query(`
		SELECT id, event_id, first_name, last_name, email, phone,
		       jamatkhana, gender, company, role, is_walkin,
		       checked_in, checked_in_at, paid, paid_at,
		       created_at, updated_at
		FROM event_participants
		WHERE event_id = ? AND (
			first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR
			company LIKE ? OR role LIKE ? OR jamatkhana LIKE ? OR
			(first_name || ' ' || last_name) LIKE ?
		)
		ORDER BY last_name ASC, first_name ASC`,
		eventID, like, like, like, like, like, like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParticipants(rows)
}

// SearchByEventName returns participants matching a name-only search query.
func (r *ParticipantRepository) SearchByEventName(eventID, query string) ([]*models.Participant, error) {
	like := "%" + query + "%"
	rows, err := r.db.Query(`
		SELECT id, event_id, first_name, last_name, email, phone,
		       jamatkhana, gender, company, role, is_walkin,
		       checked_in, checked_in_at, paid, paid_at,
		       created_at, updated_at
		FROM event_participants
		WHERE event_id = ? AND (
			first_name LIKE ? OR last_name LIKE ? OR
			(first_name || ' ' || last_name) LIKE ?
		)
		ORDER BY last_name ASC, first_name ASC`,
		eventID, like, like, like,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanParticipants(rows)
}

// CountByEvent returns total and checked-in counts for an event.
func (r *ParticipantRepository) CountByEvent(eventID string) (*models.ParticipantCounts, error) {
	c := &models.ParticipantCounts{}
	err := r.db.QueryRow(`
		SELECT
			COUNT(*),
			SUM(CASE WHEN checked_in = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN paid = 1 THEN 1 ELSE 0 END)
		FROM event_participants
		WHERE event_id = ?`, eventID,
	).Scan(&c.Total, &c.CheckedIn, &c.Paid)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// Delete removes a single participant.
func (r *ParticipantRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM event_participants WHERE id = ?`, id)
	return err
}

// SetCheckedIn toggles the checked-in status with a timestamp.
func (r *ParticipantRepository) SetCheckedIn(id string, checkedIn bool) error {
	if checkedIn {
		_, err := r.db.Exec(
			`UPDATE event_participants SET checked_in=1, checked_in_at=datetime('now'), updated_at=datetime('now') WHERE id=?`, id)
		return err
	}
	_, err := r.db.Exec(
		`UPDATE event_participants SET checked_in=0, checked_in_at=NULL, updated_at=datetime('now') WHERE id=?`, id)
	return err
}

// SetPaid toggles the paid status with a timestamp.
func (r *ParticipantRepository) SetPaid(id string, paid bool) error {
	if paid {
		_, err := r.db.Exec(
			`UPDATE event_participants SET paid=1, paid_at=datetime('now'), updated_at=datetime('now') WHERE id=?`, id)
		return err
	}
	_, err := r.db.Exec(
		`UPDATE event_participants SET paid=0, paid_at=NULL, updated_at=datetime('now') WHERE id=?`, id)
	return err
}

// DeleteByEvent removes all participants for an event.
func (r *ParticipantRepository) DeleteByEvent(eventID string) error {
	_, err := r.db.Exec(`DELETE FROM event_participants WHERE event_id = ?`, eventID)
	return err
}

// ListDistinctCompanies returns all unique non-empty company values.
func (r *ParticipantRepository) ListDistinctCompanies() ([]string, error) {
	return r.listDistinct("company")
}

// ListDistinctRoles returns all unique non-empty role values.
func (r *ParticipantRepository) ListDistinctRoles() ([]string, error) {
	return r.listDistinct("role")
}

func (r *ParticipantRepository) listDistinct(column string) ([]string, error) {
	rows, err := r.db.Query(`SELECT DISTINCT `+column+` FROM event_participants WHERE `+column+` != '' ORDER BY `+column+` ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// ListAllRegistrants returns deduplicated registrants across all events.
// Merges participants with the same name who share a phone or email.
func (r *ParticipantRepository) ListAllRegistrants(search, company, role string) ([]*models.Registrant, error) {
	return r.buildRegistrants(search, "", company, role)
}

// GetRegistrantByKey returns a single registrant by their key (first participant ID).
func (r *ParticipantRepository) GetRegistrantByKey(key string) (*models.Registrant, error) {
	registrants, err := r.buildRegistrants("", key, "", "")
	if err != nil {
		return nil, err
	}
	for _, reg := range registrants {
		if reg.Key == key {
			return reg, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *ParticipantRepository) buildRegistrants(search, keyFilter, companyFilter, roleFilter string) ([]*models.Registrant, error) {
	q := `
		SELECT p.id, p.first_name, p.last_name, p.email, p.phone,
		       p.jamatkhana, p.company, p.role,
		       p.event_id, e.name, COALESCE(e.event_date, ''),
		       p.checked_in, p.paid
		FROM event_participants p
		JOIN events e ON e.id = p.event_id
		ORDER BY p.last_name ASC, p.first_name ASC`

	rows, err := r.db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type row struct {
		id, firstName, lastName, email, phone   string
		jamatkhana, company, role               string
		eventID, eventName, eventDate           string
		checkedIn, paid                         bool
	}
	var rawRows []row
	for rows.Next() {
		var rr row
		var ci, pd int
		if err := rows.Scan(&rr.id, &rr.firstName, &rr.lastName, &rr.email, &rr.phone,
			&rr.jamatkhana, &rr.company, &rr.role,
			&rr.eventID, &rr.eventName, &rr.eventDate,
			&ci, &pd); err != nil {
			return nil, err
		}
		rr.checkedIn = ci == 1
		rr.paid = pd == 1
		rawRows = append(rawRows, rr)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Group by normalized name
	type nameKey struct{ first, last string }
	nameGroups := make(map[nameKey][]int)
	for i, rr := range rawRows {
		k := nameKey{strings.ToLower(strings.TrimSpace(rr.firstName)), strings.ToLower(strings.TrimSpace(rr.lastName))}
		nameGroups[k] = append(nameGroups[k], i)
	}

	// Union-Find
	parent := make([]int, len(rawRows))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(x int) int {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}
		return parent[x]
	}
	union := func(a, b int) {
		fa, fb := find(a), find(b)
		if fa != fb {
			parent[fa] = fb
		}
	}

	for _, indices := range nameGroups {
		emailMap := make(map[string]int)
		phoneMap := make(map[string]int)
		for _, i := range indices {
			e := strings.ToLower(strings.TrimSpace(rawRows[i].email))
			p := strings.TrimSpace(rawRows[i].phone)
			if e != "" {
				if prev, ok := emailMap[e]; ok {
					union(prev, i)
				} else {
					emailMap[e] = i
				}
			}
			if p != "" {
				if prev, ok := phoneMap[p]; ok {
					union(prev, i)
				} else {
					phoneMap[p] = i
				}
			}
			if e == "" && p == "" && len(indices) > 0 {
				union(indices[0], i)
			}
		}
	}

	// Collect clusters
	clusters := make(map[int][]int)
	for i := range rawRows {
		root := find(i)
		clusters[root] = append(clusters[root], i)
	}

	// Build registrants
	var registrants []*models.Registrant
	for _, indices := range clusters {
		emailSet := make(map[string]bool)
		phoneSet := make(map[string]bool)
		jkSet := make(map[string]bool)
		eventSet := make(map[string]bool)
		var events []models.RegistrantEvent
		var company, title string

		first := rawRows[indices[0]]
		key := first.id // stable key = first participant ID

		for _, i := range indices {
			rr := rawRows[i]
			e := strings.TrimSpace(rr.email)
			p := strings.TrimSpace(rr.phone)
			jk := strings.TrimSpace(rr.jamatkhana)
			if e != "" && !emailSet[strings.ToLower(e)] {
				emailSet[strings.ToLower(e)] = true
			}
			if p != "" && !phoneSet[p] {
				phoneSet[p] = true
			}
			if jk != "" && !jkSet[strings.ToLower(jk)] {
				jkSet[strings.ToLower(jk)] = true
			}
			// Pick first non-empty company and title across events
			if company == "" && strings.TrimSpace(rr.company) != "" {
				company = strings.TrimSpace(rr.company)
			}
			if title == "" && strings.TrimSpace(rr.role) != "" {
				title = strings.TrimSpace(rr.role)
			}
			if !eventSet[rr.eventID] {
				eventSet[rr.eventID] = true
				events = append(events, models.RegistrantEvent{
					EventID:   rr.eventID,
					EventName: rr.eventName,
					EventDate: rr.eventDate,
					CheckedIn: rr.checkedIn,
					Paid:      rr.paid,
				})
			}
		}

		var emails, phones, jamatkhanas []string
		for e := range emailSet {
			emails = append(emails, e)
		}
		for p := range phoneSet {
			phones = append(phones, p)
		}
		for jk := range jkSet {
			jamatkhanas = append(jamatkhanas, jk)
		}

		reg := &models.Registrant{
			Key:         key,
			FirstName:   first.firstName,
			LastName:    first.lastName,
			Name:        strings.TrimSpace(first.firstName + " " + first.lastName),
			Company:     company,
			Title:       title,
			Emails:      emails,
			Phones:      phones,
			Jamatkhanas: jamatkhanas,
			Events:      events,
			TotalEvents: len(events),
		}
		registrants = append(registrants, reg)
	}

	sortRegistrants(registrants)

	// If looking for a specific key, return early
	if keyFilter != "" {
		for _, reg := range registrants {
			if reg.Key == keyFilter {
				return []*models.Registrant{reg}, nil
			}
		}
		return nil, nil
	}

	// Apply filters
	if search != "" || companyFilter != "" || roleFilter != "" {
		searchLower := strings.ToLower(search)
		var filtered []*models.Registrant
		for _, reg := range registrants {
			// Search filter
			if search != "" {
				match := strings.Contains(strings.ToLower(reg.Name), searchLower)
				if !match {
					for _, e := range reg.Emails {
						if strings.Contains(strings.ToLower(e), searchLower) {
							match = true
							break
						}
					}
				}
				if !match {
					for _, p := range reg.Phones {
						if strings.Contains(p, searchLower) {
							match = true
							break
						}
					}
				}
				if !match {
					continue
				}
			}

			// Company filter
			if companyFilter != "" && !strings.EqualFold(reg.Company, companyFilter) {
				continue
			}

			// Title filter
			if roleFilter != "" && !strings.EqualFold(reg.Title, roleFilter) {
				continue
			}

			filtered = append(filtered, reg)
		}
		registrants = filtered
	}

	return registrants, nil
}

func sortRegistrants(registrants []*models.Registrant) {
	for i := 1; i < len(registrants); i++ {
		for j := i; j > 0; j-- {
			a, b := registrants[j], registrants[j-1]
			aKey := strings.ToLower(a.LastName + a.FirstName)
			bKey := strings.ToLower(b.LastName + b.FirstName)
			if aKey < bKey {
				registrants[j], registrants[j-1] = registrants[j-1], registrants[j]
			} else {
				break
			}
		}
	}
}

// helpers

func scanParticipants(rows *sql.Rows) ([]*models.Participant, error) {
	var participants []*models.Participant
	for rows.Next() {
		p := &models.Participant{}
		var isWalkin, checkedIn, paid int
		if err := rows.Scan(
			&p.ID, &p.EventID, &p.FirstName, &p.LastName, &p.Email, &p.Phone,
			&p.Jamatkhana, &p.Gender, &p.Company, &p.Role, &isWalkin,
			&checkedIn, &p.CheckedInAt, &paid, &p.PaidAt,
			&p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p.IsWalkin = isWalkin == 1
		p.CheckedIn = checkedIn == 1
		p.Paid = paid == 1
		participants = append(participants, p)
	}
	return participants, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// trimLower is a helper for CSV column matching.
func trimLower(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}
