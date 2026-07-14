package app

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type IApplications interface {
	CreateApplication(ctx context.Context, arg database.CreateApplicationParams) (database.Application, error)
	GetApplication(ctx context.Context, id int64) (database.Application, error)
	ListApplications(ctx context.Context) ([]database.Application, error)
	UpdateApplication(ctx context.Context, arg database.UpdateApplicationParams) (database.Application, error)
	DeleteApplication(ctx context.Context, id int64) error
}

type IOffersHandler interface {
	List(w http.ResponseWriter, r *http.Request)
	ShowAdd(w http.ResponseWriter, r *http.Request)
	Add(w http.ResponseWriter, r *http.Request)
}

type OffersHandler struct {
	timeout  time.Duration
	apps     IApplications
	template *template.Template
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

func NewOffersHandler(apps IApplications, tmpl *template.Template, timeout time.Duration) *OffersHandler {
	return &OffersHandler{apps: apps, template: tmpl, timeout: timeout}
}

func (h *OffersHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
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
	if err := h.template.ExecuteTemplate(w, "offers/list", data); err != nil {
		log.Printf("could not execute list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *OffersHandler) ShowAdd(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	data := listPageData{LoggedIn: true}
	if err := h.template.ExecuteTemplate(w, "offers/add", data); err != nil {
		log.Printf("could not execute list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *OffersHandler) Add(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// get data from post
	if err := r.ParseForm(); err != nil {
		log.Printf("could not parse form: %v", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	fmt.Println(r.PostForm)

	p, err := applicationParamsFromForm(r)
	if err != nil {
		log.Printf("bad form: %w", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	_, err = h.apps.CreateApplication(ctx, p)
	if err != nil {
		log.Printf("could not list applications: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// return to list with flaash msg (data from first param from create application)

	http.Redirect(w, r, "/l1st", http.StatusOK)
}

func applicationParamsFromForm(r *http.Request) (database.CreateApplicationParams, error) {
	var p database.CreateApplicationParams

	// wymagane
	p.CompanyName = strings.TrimSpace(r.PostFormValue("company_name"))
	if p.CompanyName == "" {
		return p, fmt.Errorf("company_name is required")
	}

	date, err := time.Parse("2006-01-02", r.PostFormValue("application_date"))
	if err != nil {
		return p, fmt.Errorf("invalid application_date: %w", err)
	}
	p.ApplicationDate = pgtype.Date{Time: date, Valid: true}

	// status — allowlist
	status := database.ApplicationStatus(r.PostFormValue("status"))
	switch status {
	case database.ApplicationStatusApplied, database.ApplicationStatusProgress,
		database.ApplicationStatusFailed, database.ApplicationStatusAbandon:
		p.Status = status
	default:
		return p, fmt.Errorf("invalid status: %q", status)
	}

	// checkbox: obecne pole == "true"
	p.HadChat = r.PostFormValue("had_chat") == "true"

	// opcjonalne liczby
	if p.StartingPrice, err = optInt4(r.PostFormValue("starting_price")); err != nil {
		return p, fmt.Errorf("invalid starting_price: %w", err)
	}
	if p.EndingPrice, err = optInt4(r.PostFormValue("ending_price")); err != nil {
		return p, fmt.Errorf("invalid ending_price: %w", err)
	}
	if p.MyPrice, err = optInt4(r.PostFormValue("my_price")); err != nil {
		return p, fmt.Errorf("invalid my_price: %w", err)
	}

	// opcjonalne teksty
	p.OfferLink = optText(r.PostFormValue("offer_link"))
	p.Stack = optText(r.PostFormValue("stack"))
	p.Outcome = optText(r.PostFormValue("outcome"))
	p.FixesAfterChat = optText(r.PostFormValue("fixes_after_chat"))
	p.AdditionalInfo = optText(r.PostFormValue("additional_info"))
	p.ThingsToFix = optText(r.PostFormValue("things_to_fix"))

	return p, nil
}
