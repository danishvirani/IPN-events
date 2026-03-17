package models

import "time"

// EventPhoto represents a single photo in an event's gallery.
type EventPhoto struct {
	ID        string
	EventID   string
	Filename  string // relative path under uploads, e.g. "photos/abc.jpg"
	Thumbnail string // relative path, e.g. "photos/thumbs/abc.jpg"
	Caption   string
	SortOrder int
	CreatedAt time.Time
}

// EventPhotoGroup groups photos by event for the admin gallery view.
type EventPhotoGroup struct {
	EventID   string
	EventName string
	Photos    []*EventPhoto
}
