package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"ipn-events/internal/models"
	"ipn-events/web/middleware"
)

// PageData is passed to every template.
type PageData struct {
	CurrentUser *models.User
	Flash       *Flash
	Data        any
}

type Flash struct {
	Type    string // "success" | "error"
	Message string
}

var (
	templateCache map[string]*template.Template
	tmplOnce      sync.Once
)

func loadTemplates() map[string]*template.Template {
	cache := make(map[string]*template.Template)
	base := "web/templates/layout/base.html"
	partials, _ := filepath.Glob("web/templates/partials/*.html")

	partialSet := make(map[string]bool)
	partialSet[base] = true
	for _, p := range partials {
		partialSet[p] = true
	}

	// Walk all subdirectories under web/templates to find page templates
	rootFS := os.DirFS(".")
	var pages []string
	_ = fs.WalkDir(rootFS, "web/templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		if !partialSet[path] {
			pages = append(pages, path)
		}
		return nil
	})

	for _, page := range pages {
		files := append([]string{base, page}, partials...)
		t, err := template.New(filepath.Base(page)).Funcs(templateFuncs()).ParseFiles(files...)
		if err != nil {
			log.Fatalf("render: parse %s: %v", page, err)
		}
		cache[page] = t
	}
	return cache
}

func templateFuncs() template.FuncMap {
	return template.FuncMap{
		"json": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil
		},
		"add": func(a, b int) int { return a + b },
		"dict": func(values ...any) map[string]any {
			m := make(map[string]any)
			for i := 0; i+1 < len(values); i += 2 {
				if key, ok := values[i].(string); ok {
					m[key] = values[i+1]
				}
			}
			return m
		},
		"hasInitiative": func(initiatives []models.Initiative, id string) bool {
			for _, init := range initiatives {
				if init.ID == id {
					return true
				}
			}
			return false
		},
		"formatMoney": func(cents int) string {
			negative := cents < 0
			if negative {
				cents = -cents
			}
			dollars := cents / 100
			remainder := cents % 100
			// Add comma separators
			s := fmt.Sprintf("%d", dollars)
			if len(s) > 3 {
				var parts []string
				for len(s) > 3 {
					parts = append([]string{s[len(s)-3:]}, parts...)
					s = s[:len(s)-3]
				}
				parts = append([]string{s}, parts...)
				s = strings.Join(parts, ",")
			}
			if negative {
				return fmt.Sprintf("-$%s.%02d", s, remainder)
			}
			return fmt.Sprintf("$%s.%02d", s, remainder)
		},
		"sub": func(a, b int) int { return a - b },
		"mul": func(a, b int) int { return a * b },
		"div": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"pct": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return (a * 100) / b
		},
		"dollarsToCents": func(d float64) int { return int(math.Round(d * 100)) },
		"incomeCategories":  models.IncomeCategories,
		"expenseCategories": models.ExpenseCategories,
	}
}

func render(w http.ResponseWriter, r *http.Request, templatePath string, data any) {
	tmplOnce.Do(func() {
		templateCache = loadTemplates()
	})

	t, ok := templateCache[templatePath]
	if !ok {
		log.Printf("render: template not found: %s", templatePath)
		http.Error(w, "Template not found", http.StatusInternalServerError)
		return
	}

	pd := PageData{
		CurrentUser: middleware.UserFromContext(r.Context()),
		Flash:       popFlash(w, r),
		Data:        data,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "base", pd); err != nil {
		log.Printf("render: execute %s: %v", templatePath, err)
	}
}

// renderPartial renders a template without the base layout (for HTMX partials).
func renderPartial(w http.ResponseWriter, r *http.Request, templatePath string, data any) {
	t, err := template.New(filepath.Base(templatePath)).Funcs(templateFuncs()).ParseFiles(templatePath)
	if err != nil {
		log.Printf("renderPartial: parse %s: %v", templatePath, err)
		http.Error(w, "Template error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, filepath.Base(templatePath), data); err != nil {
		log.Printf("renderPartial: execute %s: %v", templatePath, err)
	}
}

// Flash cookie helpers
func setFlash(w http.ResponseWriter, flashType, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash_type",
		Value:    flashType,
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "flash_msg",
		Value:    msg,
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func popFlash(w http.ResponseWriter, r *http.Request) *Flash {
	tc, err1 := r.Cookie("flash_type")
	mc, err2 := r.Cookie("flash_msg")
	if err1 != nil || err2 != nil {
		return nil
	}
	// Clear the flash cookies
	http.SetCookie(w, &http.Cookie{Name: "flash_type", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: "flash_msg", Path: "/", MaxAge: -1})
	return &Flash{Type: tc.Value, Message: mc.Value}
}
