package handlers

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	database "github.com/grzegab/wok/internal/database/generated"
)

type fakePossibilityStore struct {
	rows []database.ListPossibilitiesByCategoryPagedRow
	got  database.ListPossibilitiesByCategoryPagedParams
}

func (f *fakePossibilityStore) ListPossibilitiesByCategoryPaged(_ context.Context, arg database.ListPossibilitiesByCategoryPagedParams) ([]database.ListPossibilitiesByCategoryPagedRow, error) {
	f.got = arg
	return f.rows, nil
}

func serveScraped(t *testing.T, store *fakePossibilityStore, target string) string {
	t.Helper()
	tmpl := template.Must(template.New("scraped/list").Parse("{{len .Possibilities}} prev={{.PrevPage}} next={{.NextPage}}"))
	r := chi.NewRouter()
	r.Get("/scr4p3d/{category}", NewScrapedHandler(store, tmpl, time.Second).List)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	return rec.Body.String()
}

func TestScrapedList_RequestsOnePageFromOffset(t *testing.T) {
	store := &fakePossibilityStore{}

	serveScraped(t, store, "/scr4p3d/go?page=3")

	if store.got.Category != "Go" || store.got.Limit != scrapedPageSize+1 || store.got.Offset != 2*scrapedPageSize {
		t.Fatalf("query params = %+v", store.got)
	}
}

func TestScrapedList_InvalidPageFallsBackToFirst(t *testing.T) {
	store := &fakePossibilityStore{}

	body := serveScraped(t, store, "/scr4p3d/go?page=-2")

	if store.got.Offset != 0 || body != "0 prev=0 next=0" {
		t.Fatalf("offset = %d, body = %q", store.got.Offset, body)
	}
}

func TestScrapedList_ShowsNextPageOnlyWhenMoreRowsExist(t *testing.T) {
	store := &fakePossibilityStore{rows: make([]database.ListPossibilitiesByCategoryPagedRow, scrapedPageSize+1)}

	body := serveScraped(t, store, "/scr4p3d/go?page=2")

	if want := "100 prev=1 next=3"; body != want {
		t.Fatalf("body = %q, want %q", body, want)
	}
}
