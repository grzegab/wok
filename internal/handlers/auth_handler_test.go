package handlers

import (
	"context"
	"crypto/tls"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/grzegab/wok/internal/config"
	"golang.org/x/crypto/bcrypt"
)

type fakeSessionStore struct{}

func (f *fakeSessionStore) Create(context.Context, string) error { return nil }
func (f *fakeSessionStore) Revoke(context.Context, string) error { return nil }

type fakeLoginLimiter struct {
	allow  bool
	resets int
}

func (f *fakeLoginLimiter) Allow(string) bool { return f.allow }
func (f *fakeLoginLimiter) Reset(string)      { f.resets++ }

func newTestAuthHandler(t *testing.T, limiter LoginLimiter) http.Handler {
	t.Helper()
	loginFailureDelay = 0

	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("could not hash password: %v", err)
	}
	conf := &config.Config{LoginUser: "me", LoginPassHash: string(hash), SessionTTL: time.Hour}
	tmpl := template.Must(template.New("login").Parse("{{.Error}}"))

	return middleware.ClientIPFromRemoteAddr(http.HandlerFunc(NewAuthHandler(&fakeSessionStore{}, limiter, conf, tmpl).LoginPost))
}

func loginRequest(username, password string) *http.Request {
	form := url.Values{"username": {username}, "password": {password}}
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return req
}

func TestLoginPost_RateLimited_Returns429(t *testing.T) {
	handler := newTestAuthHandler(t, &fakeLoginLimiter{allow: false})
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, loginRequest("me", "secret"))

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusTooManyRequests)
	}
	if len(rec.Result().Cookies()) != 0 {
		t.Fatal("rate-limited login must not set a session cookie")
	}
}

func TestLoginPost_Success_ResetsLimiter(t *testing.T) {
	limiter := &fakeLoginLimiter{allow: true}
	handler := newTestAuthHandler(t, limiter)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, loginRequest("me", "secret"))

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if limiter.resets != 1 {
		t.Fatalf("limiter resets = %d, want 1", limiter.resets)
	}
}

func TestLoginPost_WrongPassword_KeepsLimiterCount(t *testing.T) {
	limiter := &fakeLoginLimiter{allow: true}
	handler := newTestAuthHandler(t, limiter)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, loginRequest("me", "wrong"))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	if limiter.resets != 0 {
		t.Fatal("failed login must not reset the limiter")
	}
}

func TestLoginPost_OverTLS_SetsSecureCookie(t *testing.T) {
	handler := newTestAuthHandler(t, &fakeLoginLimiter{allow: true})
	req := loginRequest("me", "secret")
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure {
		t.Fatalf("session cookie over TLS must be Secure, got %+v", cookies)
	}
}
