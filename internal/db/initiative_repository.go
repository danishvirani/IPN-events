package db

import (
	"database/sql"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type InitiativeRepository struct {
	db *sql.DB
}

func NewInitiativeRepository(db *sql.DB) *InitiativeRepository {
	return &InitiativeRepository{db: db}
}

// Create inserts a new strategic initiative.
func (r *InitiativeRepository) Create(init *models.Initiative) error {
	init.ID = uuid.New().String()
	_, err := r.db.Exec(`
		INSERT INTO strategic_initiatives (id, name, objective)
		VALUES (?, ?, ?)`,
		init.ID, init.Name, init.Objective,
	)
	return err
}

// Update modifies an initiative's name and objective.
func (r *InitiativeRepository) Update(init *models.Initiative) error {
	_, err := r.db.Exec(`
		UPDATE strategic_initiatives SET name=?, objective=?, updated_at=datetime('now')
		WHERE id=?`,
		init.Name, init.Objective, init.ID,
	)
	return err
}

// GetByID fetches a single initiative with its documents.
func (r *InitiativeRepository) GetByID(id string) (*models.Initiative, error) {
	init := &models.Initiative{}
	err := r.db.QueryRow(`
		SELECT id, name, objective, created_at, updated_at
		FROM strategic_initiatives WHERE id = ?`, id,
	).Scan(&init.ID, &init.Name, &init.Objective, &init.CreatedAt, &init.UpdatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := r.db.Query(`
		SELECT id, initiative_id, filename, original_name, created_at
		FROM initiative_documents WHERE initiative_id = ?
		ORDER BY created_at`, id,
	)
	if err != nil {
		return init, nil
	}
	defer rows.Close()
	for rows.Next() {
		var doc models.InitiativeDocument
		if err := rows.Scan(&doc.ID, &doc.InitiativeID, &doc.Filename, &doc.OriginalName, &doc.CreatedAt); err == nil {
			init.Documents = append(init.Documents, doc)
		}
	}
	return init, nil
}

// ListAll returns all initiatives ordered by name.
func (r *InitiativeRepository) ListAll() ([]*models.Initiative, error) {
	rows, err := r.db.Query(`
		SELECT id, name, objective, created_at, updated_at
		FROM strategic_initiatives ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var initiatives []*models.Initiative
	for rows.Next() {
		init := &models.Initiative{}
		if err := rows.Scan(&init.ID, &init.Name, &init.Objective, &init.CreatedAt, &init.UpdatedAt); err != nil {
			return nil, err
		}
		initiatives = append(initiatives, init)
	}
	return initiatives, rows.Err()
}

// Delete removes an initiative. CASCADE handles documents and event_initiatives.
func (r *InitiativeRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM strategic_initiatives WHERE id = ?`, id)
	return err
}

// AddDocument inserts a document record for an initiative.
func (r *InitiativeRepository) AddDocument(initiativeID, filename, originalName string) error {
	_, err := r.db.Exec(`
		INSERT INTO initiative_documents (id, initiative_id, filename, original_name)
		VALUES (?, ?, ?, ?)`,
		uuid.New().String(), initiativeID, filename, originalName,
	)
	return err
}

// DeleteDocument removes a single document record.
func (r *InitiativeRepository) DeleteDocument(docID string) error {
	_, err := r.db.Exec(`DELETE FROM initiative_documents WHERE id = ?`, docID)
	return err
}

// GetDocumentByID fetches a single document record.
func (r *InitiativeRepository) GetDocumentByID(docID string) (*models.InitiativeDocument, error) {
	doc := &models.InitiativeDocument{}
	err := r.db.QueryRow(`
		SELECT id, initiative_id, filename, original_name, created_at
		FROM initiative_documents WHERE id = ?`, docID,
	).Scan(&doc.ID, &doc.InitiativeID, &doc.Filename, &doc.OriginalName, &doc.CreatedAt)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// ListByEvent returns all initiatives tagged to a specific event.
func (r *InitiativeRepository) ListByEvent(eventID string) ([]*models.Initiative, error) {
	rows, err := r.db.Query(`
		SELECT si.id, si.name, si.objective, si.created_at, si.updated_at
		FROM strategic_initiatives si
		JOIN event_initiatives ei ON si.id = ei.initiative_id
		WHERE ei.event_id = ?
		ORDER BY si.name`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var initiatives []*models.Initiative
	for rows.Next() {
		init := &models.Initiative{}
		if err := rows.Scan(&init.ID, &init.Name, &init.Objective, &init.CreatedAt, &init.UpdatedAt); err != nil {
			return nil, err
		}
		initiatives = append(initiatives, init)
	}
	return initiatives, rows.Err()
}

// SetEventInitiatives replaces the initiative tags for an event.
func (r *InitiativeRepository) SetEventInitiatives(tx *sql.Tx, eventID string, initiativeIDs []string) error {
	if _, err := tx.Exec(`DELETE FROM event_initiatives WHERE event_id = ?`, eventID); err != nil {
		return err
	}
	for _, initID := range initiativeIDs {
		if _, err := tx.Exec(`
			INSERT INTO event_initiatives (event_id, initiative_id) VALUES (?, ?)`,
			eventID, initID,
		); err != nil {
			return err
		}
	}
	return nil
}

// SetEventInitiativesDB replaces initiative tags for an event using the db connection directly (non-transactional).
func (r *InitiativeRepository) SetEventInitiativesDB(eventID string, initiativeIDs []string) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := r.SetEventInitiatives(tx, eventID, initiativeIDs); err != nil {
		return err
	}
	return tx.Commit()
}
