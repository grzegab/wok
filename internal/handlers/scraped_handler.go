package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
)

type PossibilityStore interface {
	ListPossibilities(ctx context.Context) ([]database.Possibility, error)
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
	Url      string
	JobTitle string
	Tech     string
}

type scrapedPageData struct {
	Possibilities []possibilityRow
}

func NewScrapedHandler(store PossibilityStore, tmpl *template.Template, timeout time.Duration) *ScrapedHandler {
	return &ScrapedHandler{store: store, template: tmpl, timeout: timeout}
}

func (h *ScrapedHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	possibilities, err := h.store.ListPossibilities(ctx)
	if err != nil {
		log.Printf("could not list possibilities: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]possibilityRow, 0, len(possibilities))
	for _, p := range possibilities {
		rows = append(rows, possibilityRow{
			Url:      p.Url,
			JobTitle: p.JobTitle,
			Tech:     formatText(p.Tech),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := scrapedPageData{Possibilities: rows}
	if err := h.template.ExecuteTemplate(w, "scraped/list", data); err != nil {
		log.Printf("could not execute scraped list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
