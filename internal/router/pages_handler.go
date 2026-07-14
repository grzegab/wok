package router

//
//import (
//	"context"
//	"fmt"
//	"html/template"
//	"log"
//	"net/http"
//	"strconv"
//	"strings"
//	"time"
//
//	database "github.com/grzegab/wok/internal/database/generated"
//	"github.com/jackc/pgx/v5/pgtype"
//)
//
//const queryTimeout = 5 * time.Second
//
//type applicationLister interface {
//	ListApplications(ctx context.Context) ([]database.Application, error)
//	SaveApplication(ctx context.Context, params database.CreateApplicationParams) error
//}
//
//type PagesHandler struct {
//	tmpl *template.Template
//	apps applicationLister
//}
//
//func NewPagesHandler(tmpl *template.Template, apps applicationLister) *PagesHandler {
//	return &PagesHandler{tmpl: tmpl, apps: apps}
//}
//
//func (h *PagesHandler) landing(w http.ResponseWriter, _ *http.Request) {
//	w.Header().Set("Content-Type", "text/html; charset=utf-8")
//
//	if err := h.tmpl.ExecuteTemplate(w, "notfound", nil); err != nil {
//		log.Printf("could not execute notfound template: %v", err)
//		http.Error(w, "internal error", http.StatusInternalServerError)
//	}
//}
//
//type applicationRow struct {
//	CompanyName     string
//	ApplicationDate string
//	Status          string
//	HadChat         string
//	StartingPrice   string
//	EndingPrice     string
//	MyPrice         string
//}
//
//type listPageData struct {
//	LoggedIn     bool
//	Applications []applicationRow
//}
//
//func (h *PagesHandler) list(w http.ResponseWriter, r *http.Request) {
//	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
//	defer cancel()
//
//	apps, err := h.apps.ListApplications(ctx)
//	if err != nil {
//		log.Printf("could not list applications: %v", err)
//		http.Error(w, "internal error", http.StatusInternalServerError)
//		return
//	}
//
//	rows := make([]applicationRow, 0, len(apps))
//	for _, a := range apps {
//		rows = append(rows, applicationRow{
//			CompanyName:     a.CompanyName,
//			ApplicationDate: formatDate(a.ApplicationDate),
//			Status:          string(a.Status),
//			HadChat:         formatBool(a.HadChat),
//			StartingPrice:   formatPrice(a.StartingPrice),
//			EndingPrice:     formatPrice(a.EndingPrice),
//			MyPrice:         formatPrice(a.MyPrice),
//		})
//	}
//
//	w.Header().Set("Content-Type", "text/html; charset=utf-8")
//
//	data := listPageData{LoggedIn: true, Applications: rows}
//	if err := h.tmpl.ExecuteTemplate(w, "list", data); err != nil {
//		log.Printf("could not execute list template: %v", err)
//		http.Error(w, "internal error", http.StatusInternalServerError)
//	}
//}
//
//func (h *PagesHandler) addQuery(w http.ResponseWriter, r *http.Request) {
//	w.Header().Set("Content-Type", "text/html; charset=utf-8")
//
//	// show add form on page with back button
//	data := listPageData{LoggedIn: true}
//	if err := h.tmpl.ExecuteTemplate(w, "add", data); err != nil {
//		log.Printf("could not execute list template: %v", err)
//		http.Error(w, "internal error", http.StatusInternalServerError)
//	}
//}
//
//func (h *PagesHandler) addPost(w http.ResponseWriter, r *http.Request) {
//	ctx, cancel := context.WithTimeout(r.Context(), queryTimeout)
//	defer cancel()
//
//	// get data from post
//	if err := r.ParseForm(); err != nil {
//		log.Printf("could not parse form: %v", err)
//		http.Error(w, "invalid form", http.StatusBadRequest)
//		return
//	}
//
//	fmt.Println(r.PostForm)
//
//	p, err := applicationParamsFromForm(r)
//	if err != nil {
//		log.Printf("bad form: %w", err)
//		http.Error(w, "invalid form", http.StatusBadRequest)
//		return
//	}
//
//	err = h.apps.SaveApplication(ctx, p)
//	if err != nil {
//		log.Printf("could not list applications: %v", err)
//		http.Error(w, "internal error", http.StatusInternalServerError)
//		return
//	}
//
//	// return to list with flaash msg
//
//	http.Redirect(w, r, "/l1st", http.StatusOK)
//}
//
//func applicationParamsFromForm(r *http.Request) (database.CreateApplicationParams, error) {
//	var p database.CreateApplicationParams
//
//	// wymagane
//	p.CompanyName = strings.TrimSpace(r.PostFormValue("company_name"))
//	if p.CompanyName == "" {
//		return p, fmt.Errorf("company_name is required")
//	}
//
//	date, err := time.Parse("2006-01-02", r.PostFormValue("application_date"))
//	if err != nil {
//		return p, fmt.Errorf("invalid application_date: %w", err)
//	}
//	p.ApplicationDate = pgtype.Date{Time: date, Valid: true}
//
//	// status — allowlist
//	status := database.ApplicationStatus(r.PostFormValue("status"))
//	switch status {
//	case database.ApplicationStatusApplied, database.ApplicationStatusProgress,
//		database.ApplicationStatusFailed, database.ApplicationStatusAbandon:
//		p.Status = status
//	default:
//		return p, fmt.Errorf("invalid status: %q", status)
//	}
//
//	// checkbox: obecne pole == "true"
//	p.HadChat = r.PostFormValue("had_chat") == "true"
//
//	// opcjonalne liczby
//	if p.StartingPrice, err = optInt4(r.PostFormValue("starting_price")); err != nil {
//		return p, fmt.Errorf("invalid starting_price: %w", err)
//	}
//	if p.EndingPrice, err = optInt4(r.PostFormValue("ending_price")); err != nil {
//		return p, fmt.Errorf("invalid ending_price: %w", err)
//	}
//	if p.MyPrice, err = optInt4(r.PostFormValue("my_price")); err != nil {
//		return p, fmt.Errorf("invalid my_price: %w", err)
//	}
//
//	// opcjonalne teksty
//	p.OfferLink = optText(r.PostFormValue("offer_link"))
//	p.Stack = optText(r.PostFormValue("stack"))
//	p.Outcome = optText(r.PostFormValue("outcome"))
//	p.FixesAfterChat = optText(r.PostFormValue("fixes_after_chat"))
//	p.AdditionalInfo = optText(r.PostFormValue("additional_info"))
//	p.ThingsToFix = optText(r.PostFormValue("things_to_fix"))
//
//	return p, nil
//}
//
//// optText: pusty string -> NULL
//func optText(s string) pgtype.Text {
//	s = strings.TrimSpace(s)
//	if s == "" {
//		return pgtype.Text{}
//	}
//	return pgtype.Text{String: s, Valid: true}
//}
//
//// optInt4: pusty string -> NULL, w innym wypadku parsuj
//func optInt4(s string) (pgtype.Int4, error) {
//	s = strings.TrimSpace(s)
//	if s == "" {
//		return pgtype.Int4{}, nil
//	}
//	n, err := strconv.Atoi(s)
//	if err != nil {
//		return pgtype.Int4{}, err
//	}
//	return pgtype.Int4{Int32: int32(n), Valid: true}, nil
//}
//
//func formatDate(d pgtype.Date) string {
//	if !d.Valid {
//		return "-"
//	}
//	return d.Time.Format("2006-01-02")
//}
//
//func formatBool(b bool) string {
//	if b {
//		return "Yes"
//	}
//	return "No"
//}
//
//func formatPrice(p pgtype.Int4) string {
//	if !p.Valid {
//		return "-"
//	}
//	return fmt.Sprintf("%d", p.Int32)
//}
//
//func (h *PagesHandler) create(w http.ResponseWriter, r *http.Request) {
//	// add to database information from frontend
//	// save to DB
//	// return with view for details
//}
