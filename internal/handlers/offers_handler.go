package handlers

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ApplicationStore interface {
	CreateApplication(ctx context.Context, arg database.CreateApplicationParams) (database.Application, error)
	GetApplication(ctx context.Context, id int64) (database.Application, error)
	ListApplications(ctx context.Context) ([]database.Application, error)
	UpdateApplication(ctx context.Context, arg database.UpdateApplicationParams) (database.Application, error)
	DeleteApplication(ctx context.Context, id int64) error
}

type IOffersHandler interface {
	List(w http.ResponseWriter, r *http.Request)
	Info(w http.ResponseWriter, r *http.Request)
	ShowAdd(w http.ResponseWriter, r *http.Request)
	Add(w http.ResponseWriter, r *http.Request)
	ShowUpdateForm(w http.ResponseWriter, r *http.Request)
	Update(w http.ResponseWriter, r *http.Request)
}

type OffersHandler struct {
	timeout  time.Duration
	store    ApplicationStore
	template *template.Template
}

type applicationRow struct {
	ID              int64
	CompanyName     string
	ApplicationDate string
	Status          string
	HadChat         string
	StartingPrice   string
	EndingPrice     string
	MyPrice         string
}

type listPageData struct {
	Flash        string
	Applications []applicationRow
}

type applicationDetail struct {
	ID              int64
	CompanyName     string
	ApplicationDate string
	Status          string
	HadChat         string
	StartingPrice   string
	EndingPrice     string
	MyPrice         string
	OfferLink       string
	HasOfferLink    bool
	Stack           string
	Outcome         string
	FixesAfterChat  string
	AdditionalInfo  string
	ThingsToFix     string
	CreatedAt       string
	UpdatedAt       string
}

type applicationForm struct {
	Title           string
	Action          string
	CompanyName     string
	ApplicationDate string
	Status          string
	HadChat         bool
	StartingPrice   string
	EndingPrice     string
	MyPrice         string
	OfferLink       string
	Stack           string
	Outcome         string
	FixesAfterChat  string
	AdditionalInfo  string
	ThingsToFix     string
}

func newAddApplicationForm() applicationForm {
	return applicationForm{
		Title:  "Add application",
		Action: "/4dd",
		Status: string(database.ApplicationStatusApplied),
	}
}

func newEditApplicationForm(a database.Application) applicationForm {
	return applicationForm{
		Title:           "Edit application",
		Action:          fmt.Sprintf("/upd4te/%d", a.ID),
		CompanyName:     a.CompanyName,
		ApplicationDate: formatDate(a.ApplicationDate),
		Status:          string(a.Status),
		HadChat:         a.HadChat,
		StartingPrice:   formatPriceInput(a.StartingPrice),
		EndingPrice:     formatPriceInput(a.EndingPrice),
		MyPrice:         formatPriceInput(a.MyPrice),
		OfferLink:       a.OfferLink.String,
		Stack:           a.Stack.String,
		Outcome:         a.Outcome.String,
		FixesAfterChat:  a.FixesAfterChat.String,
		AdditionalInfo:  a.AdditionalInfo.String,
		ThingsToFix:     a.ThingsToFix.String,
	}
}

const flashCookieName = "flash"

func setFlash(w http.ResponseWriter, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Value:    url.QueryEscape(msg),
		Path:     "/",
		MaxAge:   60,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func popFlash(w http.ResponseWriter, r *http.Request) string {
	c, err := r.Cookie(flashCookieName)
	if err != nil {
		return ""
	}

	http.SetCookie(w, &http.Cookie{
		Name:     flashCookieName,
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})

	msg, err := url.QueryUnescape(c.Value)
	if err != nil {
		return ""
	}

	return msg
}

func NewOffersHandler(store ApplicationStore, tmpl *template.Template, timeout time.Duration) *OffersHandler {
	return &OffersHandler{store: store, template: tmpl, timeout: timeout}
}

func (h *OffersHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	apps, err := h.store.ListApplications(ctx)
	if err != nil {
		log.Printf("could not list applications: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows := make([]applicationRow, 0, len(apps))
	for _, a := range apps {
		rows = append(rows, applicationRow{
			ID:              a.ID,
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

	data := listPageData{Flash: popFlash(w, r), Applications: rows}
	if err := h.template.ExecuteTemplate(w, "offers/list", data); err != nil {
		log.Printf("could not execute list template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *OffersHandler) Info(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	id, err := getIdFromUrl(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	a, err := h.store.GetApplication(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("could not get application: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	data := applicationDetail{
		ID:              a.ID,
		CompanyName:     a.CompanyName,
		ApplicationDate: formatDate(a.ApplicationDate),
		Status:          string(a.Status),
		HadChat:         formatBool(a.HadChat),
		StartingPrice:   formatPrice(a.StartingPrice),
		EndingPrice:     formatPrice(a.EndingPrice),
		MyPrice:         formatPrice(a.MyPrice),
		OfferLink:       a.OfferLink.String,
		HasOfferLink:    a.OfferLink.Valid && strings.TrimSpace(a.OfferLink.String) != "",
		Stack:           formatText(a.Stack),
		Outcome:         formatText(a.Outcome),
		FixesAfterChat:  formatText(a.FixesAfterChat),
		AdditionalInfo:  formatText(a.AdditionalInfo),
		ThingsToFix:     formatText(a.ThingsToFix),
		CreatedAt:       formatTimestamp(a.CreatedAt),
		UpdatedAt:       formatTimestamp(a.UpdatedAt),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.template.ExecuteTemplate(w, "offers/details", data); err != nil {
		log.Printf("could not execute details template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *OffersHandler) ShowAdd(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if err := h.template.ExecuteTemplate(w, "offers/form", newAddApplicationForm()); err != nil {
		log.Printf("could not execute add form template: %v", err)
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

	p, err := createApplicationParams(r)
	if err != nil {
		log.Printf("bad form: %v", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	created, err := h.store.CreateApplication(ctx, p)
	if err != nil {
		log.Printf("could not create application: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setFlash(w, fmt.Sprintf("Application for %s added.", created.CompanyName))
	http.Redirect(w, r, "/l1st", http.StatusSeeOther)
}

func (h *OffersHandler) ShowUpdateForm(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	id, err := getIdFromUrl(r)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	application, err := h.store.GetApplication(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		log.Printf("could not get application: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := h.template.ExecuteTemplate(w, "offers/form", newEditApplicationForm(application)); err != nil {
		log.Printf("could not execute edit form template: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

func (h *OffersHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()

	// get data from post
	if err := r.ParseForm(); err != nil {
		log.Printf("could not parse form: %v", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	p, err := updateApplicationParams(r)
	if err != nil {
		log.Printf("bad form: %v", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	updated, err := h.store.UpdateApplication(ctx, p)
	if err != nil {
		log.Printf("could not create application: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	setFlash(w, fmt.Sprintf("Application for %s updated.", updated.CompanyName))
	http.Redirect(w, r, "/l1st", http.StatusSeeOther)
}

// applicationFields holds the form fields shared by create and update requests.
type applicationFields struct {
	CompanyName     string
	ApplicationDate pgtype.Date
	Status          database.ApplicationStatus
	HadChat         bool
	StartingPrice   pgtype.Int4
	EndingPrice     pgtype.Int4
	MyPrice         pgtype.Int4
	OfferLink       pgtype.Text
	Stack           pgtype.Text
	Outcome         pgtype.Text
	FixesAfterChat  pgtype.Text
	AdditionalInfo  pgtype.Text
	ThingsToFix     pgtype.Text
}

func parseApplicationFields(r *http.Request) (applicationFields, error) {
	var f applicationFields

	// wymagane
	f.CompanyName = strings.TrimSpace(r.PostFormValue("company_name"))
	if f.CompanyName == "" {
		return f, fmt.Errorf("company_name is required")
	}

	date, err := time.Parse("2006-01-02", r.PostFormValue("application_date"))
	if err != nil {
		return f, fmt.Errorf("invalid application_date: %w", err)
	}
	f.ApplicationDate = pgtype.Date{Time: date, Valid: true}

	// status — allowlist
	status := database.ApplicationStatus(r.PostFormValue("status"))
	switch status {
	case database.ApplicationStatusApplied, database.ApplicationStatusProgress,
		database.ApplicationStatusFailed, database.ApplicationStatusAbandon:
		f.Status = status
	default:
		return f, fmt.Errorf("invalid status: %q", status)
	}

	// checkbox: obecne pole == "true"
	f.HadChat = r.PostFormValue("had_chat") == "true"

	// opcjonalne liczby
	if f.StartingPrice, err = optInt4(r.PostFormValue("starting_price")); err != nil {
		return f, fmt.Errorf("invalid starting_price: %w", err)
	}
	if f.EndingPrice, err = optInt4(r.PostFormValue("ending_price")); err != nil {
		return f, fmt.Errorf("invalid ending_price: %w", err)
	}
	if f.MyPrice, err = optInt4(r.PostFormValue("my_price")); err != nil {
		return f, fmt.Errorf("invalid my_price: %w", err)
	}

	// opcjonalne teksty
	f.OfferLink = optText(r.PostFormValue("offer_link"))
	f.Stack = optText(r.PostFormValue("stack"))
	f.Outcome = optText(r.PostFormValue("outcome"))
	f.FixesAfterChat = optText(r.PostFormValue("fixes_after_chat"))
	f.AdditionalInfo = optText(r.PostFormValue("additional_info"))
	f.ThingsToFix = optText(r.PostFormValue("things_to_fix"))

	return f, nil
}

func createApplicationParams(r *http.Request) (database.CreateApplicationParams, error) {
	f, err := parseApplicationFields(r)
	if err != nil {
		return database.CreateApplicationParams{}, err
	}

	return database.CreateApplicationParams{
		CompanyName:     f.CompanyName,
		ApplicationDate: f.ApplicationDate,
		Status:          f.Status,
		HadChat:         f.HadChat,
		StartingPrice:   f.StartingPrice,
		EndingPrice:     f.EndingPrice,
		MyPrice:         f.MyPrice,
		OfferLink:       f.OfferLink,
		Stack:           f.Stack,
		Outcome:         f.Outcome,
		FixesAfterChat:  f.FixesAfterChat,
		AdditionalInfo:  f.AdditionalInfo,
		ThingsToFix:     f.ThingsToFix,
	}, nil
}

func updateApplicationParams(r *http.Request) (database.UpdateApplicationParams, error) {
	id, err := getIdFromUrl(r)
	if err != nil {
		return database.UpdateApplicationParams{}, err
	}

	f, err := parseApplicationFields(r)
	if err != nil {
		return database.UpdateApplicationParams{}, err
	}

	return database.UpdateApplicationParams{
		ID:              id,
		CompanyName:     f.CompanyName,
		ApplicationDate: f.ApplicationDate,
		Status:          f.Status,
		HadChat:         f.HadChat,
		StartingPrice:   f.StartingPrice,
		EndingPrice:     f.EndingPrice,
		MyPrice:         f.MyPrice,
		OfferLink:       f.OfferLink,
		Stack:           f.Stack,
		Outcome:         f.Outcome,
		FixesAfterChat:  f.FixesAfterChat,
		AdditionalInfo:  f.AdditionalInfo,
		ThingsToFix:     f.ThingsToFix,
	}, nil
}

func getIdFromUrl(r *http.Request) (int64, error) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		return 0, err
	}

	return id, nil
}
