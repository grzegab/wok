package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grzegab/wok/internal/handlers"
)

type fakeSessionRepository struct {
	valid       bool
	validateErr error
}

func (f *fakeSessionRepository) Create(ctx context.Context, token string) error {
	return nil
}

func (f *fakeSessionRepository) Validate(ctx context.Context, token string) (bool, error) {
	return f.valid, f.validateErr
}

func (f *fakeSessionRepository) Revoke(ctx context.Context, token string) error {
	return nil
}

func nextCalledHandler(called *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
	})
}

func TestRequireLogin_NoCookie_Returns404(t *testing.T) {
	repo := &fakeSessionRepository{valid: false}
	nextCalled := false
	handler := RequireLogin(repo)(nextCalledHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/somewhere", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if nextCalled {
		t.Fatalf("next.ServeHTTP should not have been called")
	}
}

func TestRequireLogin_CookiePresentValidateFalse_Returns404(t *testing.T) {
	repo := &fakeSessionRepository{valid: false}
	nextCalled := false
	handler := RequireLogin(repo)(nextCalledHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/somewhere", nil)
	req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "some-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	if nextCalled {
		t.Fatalf("next.ServeHTTP should not have been called")
	}
}

func TestRequireLogin_ValidateTrue_CallsNext(t *testing.T) {
	repo := &fakeSessionRepository{valid: true}
	nextCalled := false
	handler := RequireLogin(repo)(nextCalledHandler(&nextCalled))

	req := httptest.NewRequest(http.MethodGet, "/somewhere", nil)
	req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "valid-token"})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !nextCalled {
		t.Fatalf("next.ServeHTTP should have been called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}
