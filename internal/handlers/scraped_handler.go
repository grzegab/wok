package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/grzegab/wok/internal/category"
	database "github.com/grzegab/wok/internal/database/generated"
)

type PossibilityStore interface {
	ListPossibilitiesByCategory(ctx context.Context, category string) ([]database.Possibility, error)
}

type IScrapedHandler interface {
	List(w http.ResponseWriter, r *http.Request)
}

type ScrapedHandler struct {
	timeout  time.Duration
	store    PossibilityStore
	template *template.Template
}

type possibilityRow struct {
	Url       string
	JobTitle  string
	Tech      string
	Published string
}

type scrapedPageData struct {
	Page          string
	Category      category.Category
	Possibilities []possibilityRow
}

func NewScrapedHandler(store PossibilityStore, tmpl *template.Template, timeout time.Duration) *ScrapedHandler {
	return &ScrapedHandler{store: store, template: tmpl, timeout: timeout}
}

func (h *ScrapedHandler) List(w http.ResponseWriter, r *http.Request) {
	cat, ok := requestCategory(r)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	possibilities, err := h.store.ListPossibilitiesByCategory(ctx, cat.Name)
	if err != nil {
		log.Printf("could not list possibilities for %s: %v", cat.Name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]possibilityRow, 0, len(possibilities))
	for _, p := range possibilities {
		rows = append(rows, possibilityRow{
			Url:       p.Url,
			JobTitle:  p.JobTitle,
			Tech:      formatText(p.Tech),
			Published: formatDate(p.PostedAt),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := scrapedPageData{Page: "scraped", Category: cat, Possibilities: rows}
	if err := h.template.ExecuteTemplate(w, "scraped/list", data); err != nil {
		log.Printf("could not execute scraped list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
