package handlers

import (
	"context"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/grzegab/wok/internal/category"
	database "github.com/grzegab/wok/internal/database/generated"
)

const scrapedPageSize = 100

type PossibilityStore interface {
	ListPossibilitiesByCategoryPaged(ctx context.Context, arg database.ListPossibilitiesByCategoryPagedParams) ([]database.ListPossibilitiesByCategoryPagedRow, error)
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
	PrevPage      int
	NextPage      int
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

	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	possibilities, err := h.store.ListPossibilitiesByCategoryPaged(ctx, database.ListPossibilitiesByCategoryPagedParams{
		Category: cat.Name,
		Limit:    scrapedPageSize + 1,
		Offset:   int32((page - 1) * scrapedPageSize),
	})
	if err != nil {
		log.Printf("could not list possibilities for %s: %v", cat.Name, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := scrapedPageData{Page: "scraped", Category: cat, PrevPage: page - 1}
	if len(possibilities) > scrapedPageSize {
		possibilities = possibilities[:scrapedPageSize]
		data.NextPage = page + 1
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

	data.Possibilities = rows
	if err := h.template.ExecuteTemplate(w, "scraped/list", data); err != nil {
		log.Printf("could not execute scraped list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}
