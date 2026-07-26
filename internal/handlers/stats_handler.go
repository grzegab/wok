package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
)

type PossibilityStatsStore interface {
	StatsPossibility(ctx context.Context) ([]database.StatsPossibilityRow, error)
}

type IStatsHandler interface {
	Stats(w http.ResponseWriter, r *http.Request)
}

type StatsHandler struct {
	timeout  time.Duration
	store    PossibilityStatsStore
	template *template.Template
}

type technologyRow struct {
	Technology string
	Count      int64
}

type statsPageData struct {
	Technologies []technologyRow
}

func NewStatsHandler(store PossibilityStatsStore, tmpl *template.Template, timeout time.Duration) *StatsHandler {
	return &StatsHandler{store: store, template: tmpl, timeout: timeout}
}

func (h *StatsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	stats, err := h.store.StatsPossibility(ctx)
	if err != nil {
		log.Printf("could not get possibility stats: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]technologyRow, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, technologyRow{Technology: s.Technology, Count: s.Count})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := statsPageData{Technologies: rows}
	if err := h.template.ExecuteTemplate(w, "stats/stats", data); err != nil {
		log.Printf("could not execute stats template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
