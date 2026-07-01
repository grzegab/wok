package router

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/grzegab/wok/internal/config"
	"github.com/grzegab/wok/internal/templates"
	"golang.org/x/crypto/bcrypt"
)

func TestMain(m *testing.M) {
	loginFailureDelay = 0
	os.Exit(m.Run())
}

func testTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := template.ParseFS(templates.FS, "base.gohtml", "partials/*.gohtml")
	if err != nil {
		t.Fatalf("template.ParseFS: %v", err)
	}
	return tmpl
}

type fakeAuthSessionRepository struct {
	createCalled bool
	revokeCalled bool
	revokedToken string
}

func (f *fakeAuthSessionRepository) Create(ctx context.Context, token string) error {
	f.createCalled = true
	return nil
}

func (f *fakeAuthSessionRepository) Validate(ctx context.Context, token string) (bool, error) {
	return true, nil
}

func (f *fakeAuthSessionRepository) Revoke(ctx context.Context, token string) error {
	f.revokeCalled = true
	f.revokedToken = token
	return nil
}

func testConfig(t *testing.T) *config.Config {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	return &config.Config{
		LoginUser:     "alice",
		LoginPassHash: string(hash),
	}
}

func TestLogin_Get_RendersWithEmptyError(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	req := httptest.NewRequest(http.MethodGet, "/l0g1n", nil)
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	if strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected empty error region on first GET, got body containing error: %s", body)
	}
}

func TestLogin_Post_CorrectCredentials_SetsCookieAndRedirects(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	form := strings.NewReader("username=alice&password=correct-password")
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/" {
		t.Fatalf("Location = %q, want %q", loc, "/")
	}
	if !repo.createCalled {
		t.Fatalf("expected sessionRepo.Create to be called")
	}

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie, got %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName {
		t.Fatalf("cookie name = %q, want %q", cookie.Name, sessionCookieName)
	}
	if !cookie.HttpOnly {
		t.Fatalf("expected HttpOnly cookie")
	}
	if !cookie.Secure {
		t.Fatalf("expected Secure cookie")
	}
	if cookie.SameSite != http.SameSiteStrictMode {
		t.Fatalf("SameSite = %v, want Strict", cookie.SameSite)
	}
	if cookie.MaxAge != int(conf.SessionTTL.Seconds()) {
		t.Fatalf("MaxAge = %d, want %d", cookie.MaxAge, int(conf.SessionTTL.Seconds()))
	}
}

func TestLogin_Post_WrongPassword_ReRendersWithErrorAnd401(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	form := strings.NewReader("username=alice&password=wrong-password")
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected error message in body, got: %s", body)
	}
	if strings.Contains(body, "wrong-password") {
		t.Fatalf("password must not be echoed back, got: %s", body)
	}
	if repo.createCalled {
		t.Fatalf("sessionRepo.Create should not be called on failed login")
	}
}

func TestLogin_Post_WrongUsername_BehavesIdenticallyToWrongPassword(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	form := strings.NewReader("username=bob&password=correct-password")
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected generic error message in body, got: %s", body)
	}
	if repo.createCalled {
		t.Fatalf("sessionRepo.Create should not be called on failed login")
	}
}

func TestLogout_RevokesAndClearsCookieAndRedirects(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "some-token"})
	rec := httptest.NewRecorder()

	handler.logout(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if loc := rec.Header().Get("Location"); loc != "/l0g1n" {
		t.Fatalf("Location = %q, want %q", loc, "/l0g1n")
	}
	if !repo.revokeCalled {
		t.Fatalf("expected sessionRepo.Revoke to be called")
	}
	if repo.revokedToken != "some-token" {
		t.Fatalf("revoked token = %q, want %q", repo.revokedToken, "some-token")
	}

	res := rec.Result()
	cookies := res.Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie, got %d", len(cookies))
	}
	if cookies[0].MaxAge != -1 {
		t.Fatalf("MaxAge = %d, want -1", cookies[0].MaxAge)
	}
}

func TestLogout_ClearedCookie_MatchesLoginCookieNameAndPath(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	// Obtain the login-set cookie's name/path as the reference shape.
	form := strings.NewReader("username=alice&password=correct-password")
	loginReq := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	handler.login(loginRec, loginReq)
	loginCookie := loginRec.Result().Cookies()[0]

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "some-token"})
	rec := httptest.NewRecorder()

	handler.logout(rec, req)

	clearedCookie := rec.Result().Cookies()[0]
	if clearedCookie.Name != loginCookie.Name {
		t.Fatalf("cleared cookie name = %q, want %q (symmetry with login cookie)", clearedCookie.Name, loginCookie.Name)
	}
	if clearedCookie.Path != loginCookie.Path {
		t.Fatalf("cleared cookie path = %q, want %q (symmetry with login cookie)", clearedCookie.Path, loginCookie.Path)
	}
}

func TestLogin_Post_EmptyUsername_RejectedBeforeBcryptCompare(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	form := strings.NewReader("username=&password=correct-password")
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected generic error message in body, got: %s", body)
	}
	if repo.createCalled {
		t.Fatalf("sessionRepo.Create should not be called for empty username")
	}
}

func TestLogin_Post_EmptyPassword_RejectedBeforeBcryptCompare(t *testing.T) {
	repo := &fakeAuthSessionRepository{}
	conf := testConfig(t)
	handler := NewAuthHandler(repo, conf, testTemplate(t))

	form := strings.NewReader("username=alice&password=")
	req := httptest.NewRequest(http.MethodPost, "/l0g1n", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	handler.login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Invalid credentials") {
		t.Fatalf("expected generic error message in body, got: %s", body)
	}
	if repo.createCalled {
		t.Fatalf("sessionRepo.Create should not be called for empty password")
	}
}
