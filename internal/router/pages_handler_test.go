package router

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/jackc/pgx/v5/pgtype"
)

type fakeApplicationRepository struct {
	apps []database.Application
	err  error
}

func (f *fakeApplicationRepository) ListApplications(ctx context.Context) ([]database.Application, error) {
	return f.apps, f.err
}

func TestList_RendersApplicationsFromRepository(t *testing.T) {
	repo := &fakeApplicationRepository{
		apps: []database.Application{
			{
				CompanyName:     "Acme Corp",
				ApplicationDate: pgtype.Date{Time: time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), Valid: true},
				Status:          database.ApplicationStatusApplied,
				HadChat:         true,
				StartingPrice:   pgtype.Int4{Int32: 10000, Valid: true},
				EndingPrice:     pgtype.Int4{Int32: 11000, Valid: true},
				MyPrice:         pgtype.Int4{Int32: 12000, Valid: true},
			},
		},
	}
	handler := NewPagesHandler(testTemplate(t), repo)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rec := httptest.NewRecorder()

	handler.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Acme Corp") {
		t.Fatalf("expected body to contain company name, got: %s", body)
	}
	if !strings.Contains(body, "2026-06-15") {
		t.Fatalf("expected body to contain formatted date, got: %s", body)
	}
	if !strings.Contains(body, "10000") || !strings.Contains(body, "11000") {
		t.Fatalf("expected body to contain starting/ending price, got: %s", body)
	}
	if !strings.Contains(body, "Add") {
		t.Fatalf("expected body to contain an add button/link, got: %s", body)
	}
}

func TestList_EmptyRepository_RendersWithoutRows(t *testing.T) {
	repo := &fakeApplicationRepository{apps: nil}
	handler := NewPagesHandler(testTemplate(t), repo)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rec := httptest.NewRecorder()

	handler.list(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestList_RepositoryError_Returns500(t *testing.T) {
	repo := &fakeApplicationRepository{err: errors.New("connection lost")}
	handler := NewPagesHandler(testTemplate(t), repo)

	req := httptest.NewRequest(http.MethodGet, "/list", nil)
	rec := httptest.NewRecorder()

	handler.list(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if strings.Contains(rec.Body.String(), "connection lost") {
		t.Fatalf("internal error details must not leak to response body, got: %s", rec.Body.String())
	}
}
