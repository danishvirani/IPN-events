package handlers

import (
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	"ipn-events/internal/db"
	"ipn-events/internal/models"
)

type AdminInitiativeHandler struct {
	initiativeRepo *db.InitiativeRepository
	uploadDir      string
}

func NewAdminInitiativeHandler(initiativeRepo *db.InitiativeRepository, uploadDir string) *AdminInitiativeHandler {
	return &AdminInitiativeHandler{initiativeRepo: initiativeRepo, uploadDir: uploadDir}
}

// List renders all strategic initiatives.
func (h *AdminInitiativeHandler) List(w http.ResponseWriter, r *http.Request) {
	initiatives, err := h.initiativeRepo.ListAll()
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	render(w, r, "web/templates/admin/initiatives.html", initiatives)
}

// NewForm renders the create initiative form.
func (h *AdminInitiativeHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	render(w, r, "web/templates/admin/initiative_new.html", nil)
}

// Create handles the create form submission.
func (h *AdminInitiativeHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	objective := strings.TrimSpace(r.FormValue("objective"))

	if name == "" || objective == "" {
		setFlash(w, "error", "Name and objective are required.")
		http.Redirect(w, r, "/admin/initiatives/new", http.StatusSeeOther)
		return
	}

	init := &models.Initiative{
		Name:      name,
		Objective: objective,
	}
	if err := h.initiativeRepo.Create(init); err != nil {
		setFlash(w, "error", "Failed to create initiative.")
		http.Redirect(w, r, "/admin/initiatives/new", http.StatusSeeOther)
		return
	}

	// Handle file uploads (multiple)
	files := r.MultipartForm.File["documents"]
	for _, fh := range files {
		if err := h.saveDocument(init.ID, fh); err != nil {
			setFlash(w, "error", fmt.Sprintf("Initiative created but failed to upload %s: %v", fh.Filename, err))
			http.Redirect(w, r, "/admin/initiatives/"+init.ID, http.StatusSeeOther)
			return
		}
	}

	setFlash(w, "success", "Strategic initiative created.")
	http.Redirect(w, r, "/admin/initiatives/"+init.ID, http.StatusSeeOther)
}

// Show renders a single initiative with its documents.
func (h *AdminInitiativeHandler) Show(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	init, err := h.initiativeRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "web/templates/admin/initiative_show.html", init)
}

// EditForm renders the edit form for an initiative.
func (h *AdminInitiativeHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	init, err := h.initiativeRepo.GetByID(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	render(w, r, "web/templates/admin/initiative_edit.html", init)
}

// Update handles the edit form submission.
func (h *AdminInitiativeHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	name := strings.TrimSpace(r.FormValue("name"))
	objective := strings.TrimSpace(r.FormValue("objective"))

	if name == "" || objective == "" {
		setFlash(w, "error", "Name and objective are required.")
		http.Redirect(w, r, "/admin/initiatives/"+id+"/edit", http.StatusSeeOther)
		return
	}

	init := &models.Initiative{
		ID:        id,
		Name:      name,
		Objective: objective,
	}
	if err := h.initiativeRepo.Update(init); err != nil {
		setFlash(w, "error", "Failed to update initiative.")
		http.Redirect(w, r, "/admin/initiatives/"+id+"/edit", http.StatusSeeOther)
		return
	}

	setFlash(w, "success", "Initiative updated.")
	http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
}

// Delete removes an initiative.
func (h *AdminInitiativeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.initiativeRepo.Delete(id); err != nil {
		setFlash(w, "error", "Failed to delete initiative.")
		http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
		return
	}
	setFlash(w, "success", "Initiative deleted.")
	http.Redirect(w, r, "/admin/initiatives", http.StatusSeeOther)
}

// UploadDocument adds a document to an existing initiative.
func (h *AdminInitiativeHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["documents"]
	if len(files) == 0 {
		setFlash(w, "error", "No file selected.")
		http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
		return
	}

	for _, fh := range files {
		if err := h.saveDocument(id, fh); err != nil {
			setFlash(w, "error", fmt.Sprintf("Failed to upload %s: %v", fh.Filename, err))
			http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
			return
		}
	}

	setFlash(w, "success", fmt.Sprintf("Uploaded %d document(s).", len(files)))
	http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
}

// DeleteDocument removes a single document.
func (h *AdminInitiativeHandler) DeleteDocument(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	docID := chi.URLParam(r, "docId")

	// Get the doc to find its filename for disk cleanup
	doc, err := h.initiativeRepo.GetDocumentByID(docID)
	if err != nil {
		setFlash(w, "error", "Document not found.")
		http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
		return
	}

	if err := h.initiativeRepo.DeleteDocument(docID); err != nil {
		setFlash(w, "error", "Failed to delete document.")
		http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
		return
	}

	// Best-effort cleanup of the file on disk
	_ = os.Remove(filepath.Join(h.uploadDir, doc.Filename))

	setFlash(w, "success", "Document deleted.")
	http.Redirect(w, r, "/admin/initiatives/"+id, http.StatusSeeOther)
}

// saveDocument persists a single uploaded file to disk and records it in the database.
func (h *AdminInitiativeHandler) saveDocument(initiativeID string, fh *multipart.FileHeader) error {
	file, err := fh.Open()
	if err != nil {
		return fmt.Errorf("could not open file")
	}
	defer file.Close()

	if fh.Size > 32<<20 {
		return fmt.Errorf("file must be under 32 MB")
	}

	ext := strings.ToLower(filepath.Ext(fh.Filename))
	filename := randomHex(16) + ext

	dst, err := os.Create(filepath.Join(h.uploadDir, filename))
	if err != nil {
		return fmt.Errorf("could not save file")
	}
	defer dst.Close()
	if _, err := io.Copy(dst, file); err != nil {
		return fmt.Errorf("could not save file")
	}

	return h.initiativeRepo.AddDocument(initiativeID, filename, fh.Filename)
}
