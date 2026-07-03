package router

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

const listQueryTimeout = 5 * time.Second

type applicationLister interface {
	ListApplications(ctx context.Context) ([]database.Application, error)
}

type PagesHandler struct {
	tmpl *template.Template
	apps applicationLister
}

func NewPagesHandler(tmpl *template.Template, apps applicationLister) *PagesHandler {
	return &PagesHandler{tmpl: tmpl, apps: apps}
}

func (h *PagesHandler) landing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.tmpl.ExecuteTemplate(w, "notfound", nil); err != nil {
		log.Printf("could not execute notfound template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

type applicationRow struct {
	CompanyName     string
	ApplicationDate string
	Status          string
	HadChat         string
	StartingPrice   string
	EndingPrice     string
	MyPrice         string
}

type listPageData struct {
	LoggedIn     bool
	Applications []applicationRow
}

func (h *PagesHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), listQueryTimeout)
	defer cancel()

	apps, err := h.apps.ListApplications(ctx)
	if err != nil {
		log.Printf("could not list applications: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]applicationRow, 0, len(apps))
	for _, a := range apps {
		rows = append(rows, applicationRow{
			CompanyName:     a.CompanyName,
			ApplicationDate: formatDate(a.ApplicationDate),
			Status:          string(a.Status),
			HadChat:         formatBool(a.HadChat),
			StartingPrice:   formatPrice(a.StartingPrice),
			EndingPrice:     formatPrice(a.EndingPrice),
			MyPrice:         formatPrice(a.MyPrice),
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := listPageData{LoggedIn: true, Applications: rows}
	if err := h.tmpl.ExecuteTemplate(w, "list", data); err != nil {
		log.Printf("could not execute list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func formatDate(d pgtype.Date) string {
	if !d.Valid {
		return "-"
	}
	return d.Time.Format("2006-01-02")
}

func formatBool(b bool) string {
	if b {
		return "Yes"
	}
	return "No"
}

func formatPrice(p pgtype.Int4) string {
	if !p.Valid {
		return "-"
	}
	return fmt.Sprintf("%d", p.Int32)
}

func (h *PagesHandler) create(w http.ResponseWriter, r *http.Request) {
	// add to database information from frontend
	// save to DB
	// return with view for details
}
