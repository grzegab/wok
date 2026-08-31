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

type PossibilityStatsStore interface {
	StatsPossibilityByCategory(ctx context.Context, category string) ([]database.StatsPossibilityByCategoryRow, error)
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
	Page         string
	Category     category.Category
	Technologies []technologyRow
}

func NewStatsHandler(store PossibilityStatsStore, tmpl *template.Template, timeout time.Duration) *StatsHandler {
	return &StatsHandler{store: store, template: tmpl, timeout: timeout}
}

func (h *StatsHandler) Stats(w http.ResponseWriter, r *http.Request) {
	cat, ok := requestCategory(r)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	stats, err := h.store.StatsPossibilityByCategory(ctx, cat.Name)
	if err != nil {
		log.Printf("could not get possibility stats for %s: %v", cat.Name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]technologyRow, 0, len(stats))
	for _, s := range stats {
		rows = append(rows, technologyRow{Technology: s.Technology, Count: s.Count})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := statsPageData{Page: "stats", Category: cat, Technologies: rows}
	if err := h.template.ExecuteTemplate(w, "stats/stats", data); err != nil {
		log.Printf("could not execute stats template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
