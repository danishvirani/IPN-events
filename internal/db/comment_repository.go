package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"

	"ipn-events/internal/models"
)

type CommentRepository struct {
	db *sql.DB
}

func NewCommentRepository(db *sql.DB) *CommentRepository {
	return &CommentRepository{db: db}
}

// Create inserts a new comment.
func (r *CommentRepository) Create(c *models.EventComment) error {
	c.ID = uuid.New().String()
	c.CreatedAt = time.Now()
	_, err := r.db.Exec(
		`INSERT INTO event_comments (id, event_id, user_id, user_name, comment, type, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.EventID, c.UserID, c.UserName, c.Comment, c.Type, c.CreatedAt,
	)
	return err
}

// ListByEvent returns all comments for an event, ordered chronologically.
func (r *CommentRepository) ListByEvent(eventID string) ([]*models.EventComment, error) {
	rows, err := r.db.Query(
		`SELECT id, event_id, user_id, user_name, comment, type, created_at
		 FROM event_comments
		 WHERE event_id = ?
		 ORDER BY created_at ASC`,
		eventID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []*models.EventComment
	for rows.Next() {
		c := &models.EventComment{}
		if err := rows.Scan(&c.ID, &c.EventID, &c.UserID, &c.UserName,
			&c.Comment, &c.Type, &c.CreatedAt); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, rows.Err()
}

// DeleteByEvent removes all comments for an event.
func (r *CommentRepository) DeleteByEvent(eventID string) error {
	_, err := r.db.Exec(`DELETE FROM event_comments WHERE event_id = ?`, eventID)
	return err
}
