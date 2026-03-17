package db

import (
	"database/sql"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type PhotoRepository struct {
	db *sql.DB
}

func NewPhotoRepository(db *sql.DB) *PhotoRepository {
	return &PhotoRepository{db: db}
}

func (r *PhotoRepository) Create(p *models.EventPhoto) error {
	p.ID = uuid.New().String()
	_, err := r.db.Exec(`
		INSERT INTO event_photos (id, event_id, filename, thumbnail, caption, sort_order)
		VALUES (?, ?, ?, ?, ?, ?)`,
		p.ID, p.EventID, p.Filename, p.Thumbnail, p.Caption, p.SortOrder,
	)
	return err
}

func (r *PhotoRepository) ListByEvent(eventID string) ([]*models.EventPhoto, error) {
	rows, err := r.db.Query(`
		SELECT id, event_id, filename, thumbnail, caption, sort_order, created_at
		FROM event_photos WHERE event_id = ?
		ORDER BY sort_order ASC, created_at ASC`, eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []*models.EventPhoto
	for rows.Next() {
		p := &models.EventPhoto{}
		if err := rows.Scan(&p.ID, &p.EventID, &p.Filename, &p.Thumbnail, &p.Caption, &p.SortOrder, &p.CreatedAt); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// ListAllGrouped returns all photos grouped by event, for the admin gallery page.
func (r *PhotoRepository) ListAllGrouped() ([]*models.EventPhotoGroup, error) {
	rows, err := r.db.Query(`
		SELECT p.id, p.event_id, e.name, p.filename, p.thumbnail, p.caption, p.sort_order, p.created_at
		FROM event_photos p
		JOIN events e ON e.id = p.event_id
		ORDER BY e.name ASC, p.sort_order ASC, p.created_at ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupMap := make(map[string]*models.EventPhotoGroup)
	var order []string
	for rows.Next() {
		p := &models.EventPhoto{}
		var eventName string
		if err := rows.Scan(&p.ID, &p.EventID, &eventName, &p.Filename, &p.Thumbnail, &p.Caption, &p.SortOrder, &p.CreatedAt); err != nil {
			return nil, err
		}
		g, ok := groupMap[p.EventID]
		if !ok {
			g = &models.EventPhotoGroup{EventID: p.EventID, EventName: eventName}
			groupMap[p.EventID] = g
			order = append(order, p.EventID)
		}
		g.Photos = append(g.Photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var groups []*models.EventPhotoGroup
	for _, id := range order {
		groups = append(groups, groupMap[id])
	}
	return groups, nil
}

func (r *PhotoRepository) GetByID(id string) (*models.EventPhoto, error) {
	p := &models.EventPhoto{}
	err := r.db.QueryRow(`
		SELECT id, event_id, filename, thumbnail, caption, sort_order, created_at
		FROM event_photos WHERE id = ?`, id,
	).Scan(&p.ID, &p.EventID, &p.Filename, &p.Thumbnail, &p.Caption, &p.SortOrder, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PhotoRepository) Delete(id string) error {
	_, err := r.db.Exec(`DELETE FROM event_photos WHERE id = ?`, id)
	return err
}
