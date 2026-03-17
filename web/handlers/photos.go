package handlers

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/imaging"
	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

type PhotoHandler struct {
	eventRepo *db.EventRepository
	photoRepo *db.PhotoRepository
	uploadDir string
}

func NewPhotoHandler(eventRepo *db.EventRepository, photoRepo *db.PhotoRepository, uploadDir string) *PhotoHandler {
	return &PhotoHandler{eventRepo: eventRepo, photoRepo: photoRepo, uploadDir: uploadDir}
}

// Upload handles multi-file photo upload for an event.
func (h *PhotoHandler) Upload(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !user.IsAdmin() && e.UserID != user.ID && e.AssignedToID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		setFlash(w, "error", "Upload too large.")
		http.Redirect(w, r, h.eventURL(id), http.StatusSeeOther)
		return
	}

	files := r.MultipartForm.File["photos"]
	if len(files) == 0 {
		setFlash(w, "error", "No photos selected.")
		http.Redirect(w, r, h.eventURL(id), http.StatusSeeOther)
		return
	}

	uploaded := 0
	for _, fh := range files {
		if fh.Size > 10<<20 {
			continue // skip files over 10MB
		}
		ext := strings.ToLower(filepath.Ext(fh.Filename))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
			continue
		}

		file, err := fh.Open()
		if err != nil {
			continue
		}

		filename, thumbnail, err := imaging.ProcessUpload(file, h.uploadDir)
		file.Close()
		if err != nil {
			log.Printf("photo upload: process error: %v", err)
			continue
		}

		photo := &models.EventPhoto{
			EventID:   id,
			Filename:  filename,
			Thumbnail: thumbnail,
		}
		if err := h.photoRepo.Create(photo); err != nil {
			log.Printf("photo upload: db error: %v", err)
			continue
		}
		uploaded++
	}

	if uploaded > 0 {
		setFlash(w, "success", fmt.Sprintf("Uploaded %d photo(s).", uploaded))
	} else {
		setFlash(w, "error", "No valid photos were uploaded. Use JPG or PNG under 10 MB.")
	}
	http.Redirect(w, r, h.eventURL(id)+"?tab=post-event", http.StatusSeeOther)
}

// Delete removes a photo from an event.
func (h *PhotoHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	photoId := chi.URLParam(r, "photoId")
	user := middleware.UserFromContext(r.Context())

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if !user.IsAdmin() && e.UserID != user.ID && e.AssignedToID != user.ID {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	photo, err := h.photoRepo.GetByID(photoId)
	if err != nil || photo.EventID != id {
		http.NotFound(w, r)
		return
	}

	_ = h.photoRepo.Delete(photoId)
	_ = os.Remove(filepath.Join(h.uploadDir, photo.Filename))
	_ = os.Remove(filepath.Join(h.uploadDir, photo.Thumbnail))

	setFlash(w, "success", "Photo deleted.")
	http.Redirect(w, r, h.eventURL(id)+"?tab=post-event", http.StatusSeeOther)
}

// ToggleComplete marks or unmarks an event as completed.
func (h *PhotoHandler) ToggleComplete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middleware.UserFromContext(r.Context())

	if !user.IsAdmin() {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	e, err := h.eventRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	_ = h.eventRepo.ToggleCompleted(id, !e.Completed)

	setFlash(w, "success", "Event completion status updated.")
	http.Redirect(w, r, h.eventURL(id)+"?tab=post-event", http.StatusSeeOther)
}

// Gallery renders the admin photo gallery page showing all event photos.
func (h *PhotoHandler) Gallery(w http.ResponseWriter, r *http.Request) {
	groups, err := h.photoRepo.ListAllGrouped()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/gallery.html", groups)
}

func (h *PhotoHandler) eventURL(id string) string {
	return "/events/" + id
}
