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

type fakeStatsStore struct {
	rows []database.TechTrendByCategoryRow
	got  database.TechTrendByCategoryParams
}

func (f *fakeStatsStore) TechTrendByCategory(_ context.Context, arg database.TechTrendByCategoryParams) ([]database.TechTrendByCategoryRow, error) {
	f.got = arg
	return f.rows, nil
}

func serveStats(t *testing.T, store *fakeStatsStore, target string) *httptest.ResponseRecorder {
	t.Helper()
	tmpl := template.Must(template.New("stats/stats").Parse(
		"{{.Days}} {{.CurrentTotal}}/{{.PreviousTotal}}{{range .Technologies}} {{.Technology}}:{{.Change}}{{end}}"))
	r := chi.NewRouter()
	r.Get("/st4ts/{category}", NewStatsHandler(store, tmpl, time.Second).Stats)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestStats_DefaultsToThirtyDaysComparedWithPreviousThirty(t *testing.T) {
	store := &fakeStatsStore{}

	rec := serveStats(t, store, "/st4ts/go")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	current, previous := store.got.CurrentStart.Time, store.got.PreviousStart.Time
	if store.got.Category != "Go" || current.Sub(previous) < 29*24*time.Hour || current.Sub(previous) > 31*24*time.Hour {
		t.Fatalf("query params = %+v", store.got)
	}
	if time.Since(current) < 29*24*time.Hour {
		t.Fatalf("current period starts at %v, want about 30 days ago", current)
	}
}

func TestStats_ShowsShareChangeBetweenPeriods(t *testing.T) {
	store := &fakeStatsStore{rows: []database.TechTrendByCategoryRow{
		{Technology: "python", CurrentTotal: 100, PreviousTotal: 50, CurrentShare: 30, PreviousShare: 40},
		{Technology: "java", CurrentTotal: 100, PreviousTotal: 50, CurrentShare: 8.5, PreviousShare: 4},
	}}

	rec := serveStats(t, store, "/st4ts/go?days=90")

	if want := "90 100/50 python:-10 java:4.5"; rec.Body.String() != want {
		t.Fatalf("body = %q, want %q", rec.Body.String(), want)
	}
}

func TestStats_RejectsPeriodOutsideAllowlist(t *testing.T) {
	rec := serveStats(t, &fakeStatsStore{}, "/st4ts/go?days=45")

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
