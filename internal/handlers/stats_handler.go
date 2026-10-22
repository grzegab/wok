package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/grzegab/wok/internal/category"
	database "github.com/grzegab/wok/internal/database/generated"
)

const defaultStatsPeriodDays = 30

var statsPeriodDays = []int{7, 30, 90}

type PossibilityStatsStore interface {
	TechTrendByCategory(ctx context.Context, arg database.TechTrendByCategoryParams) ([]database.TechTrendByCategoryRow, error)
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
	Technology    string
	CurrentCount  int64
	PreviousCount int64
	CurrentShare  float64
	PreviousShare float64
	Change        float64
}

type statsPageData struct {
	Page          string
	Category      category.Category
	Days          int
	Periods       []int
	CurrentTotal  int64
	PreviousTotal int64
	Technologies  []technologyRow
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

	days, ok := statsPeriod(r.URL.Query().Get("days"))
	if !ok {
		http.Error(w, "days must be one of 7, 30, 90", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	currentStart := time.Now().AddDate(0, 0, -days)
	stats, err := h.store.TechTrendByCategory(ctx, database.TechTrendByCategoryParams{
		Category:      cat.Name,
		CurrentStart:  pgtype.Timestamptz{Time: currentStart, Valid: true},
		PreviousStart: pgtype.Timestamptz{Time: currentStart.AddDate(0, 0, -days), Valid: true},
	})
	if err != nil {
		log.Printf("could not get technology trend for %s: %v", cat.Name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := statsPageData{Page: "stats", Category: cat, Days: days, Periods: statsPeriodDays}
	data.Technologies = make([]technologyRow, 0, len(stats))
	for _, s := range stats {
		data.CurrentTotal, data.PreviousTotal = s.CurrentTotal, s.PreviousTotal
		data.Technologies = append(data.Technologies, technologyRow{
			Technology:    s.Technology,
			CurrentCount:  s.CurrentCount,
			PreviousCount: s.PreviousCount,
			CurrentShare:  s.CurrentShare,
			PreviousShare: s.PreviousShare,
			Change:        s.CurrentShare - s.PreviousShare,
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.template.ExecuteTemplate(w, "stats/stats", data); err != nil {
		log.Printf("could not execute stats template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func statsPeriod(value string) (int, bool) {
	if value == "" {
		return defaultStatsPeriodDays, true
	}
	days, err := strconv.Atoi(value)
	if err != nil || !slices.Contains(statsPeriodDays, days) {
		return 0, false
	}
	return days, true
}
