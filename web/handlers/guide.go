package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

type GuideHandler struct{}

func NewGuideHandler() *GuideHandler { return &GuideHandler{} }

func (h *GuideHandler) Show(w http.ResponseWriter, r *http.Request) {
	role := chi.URLParam(r, "role")
	switch role {
	case "member":
		render(w, r, "web/templates/guide/member.html", nil)
	case "viewer":
		render(w, r, "web/templates/guide/viewer.html", nil)
	case "admin":
		render(w, r, "web/templates/guide/admin.html", nil)
	default:
		http.NotFound(w, r)
	}
}
