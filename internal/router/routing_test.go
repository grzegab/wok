package router

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/grzegab/wok/internal/config"
	"github.com/grzegab/wok/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

// TestRouting_LoginAndLogoutNotInterceptedByCatchAllBan guards the Chi routing
// precedence: explicit POST /l0g1n and POST /logout registrations must resolve
// ahead of the catch-all POST /* ban route, even though both /l0g1n and /logout
// are also session-exempt paths in RequireLogin.
func TestRouting_LoginAndLogoutNotInterceptedByCatchAllBan(t *testing.T) {
	ipRepo := repository.NewInMemoryIpRepository()
	sessionRepo := &fakeSessionRepository{valid: false}
	authHdlr := NewAuthHandler(sessionRepo, &config.Config{}, testTemplate(t))
	pagesHdlr := NewPagesHandler(testTemplate(t))

	r := New(ipRepo, sessionRepo, authHdlr, pagesHdlr)

	tests := []struct {
		name string
		path string
	}{
		{"login", "/l0g1n"},
		{"logout", "/logout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tt.path, nil)
			w := httptest.NewRecorder()

			r.Mux().ServeHTTP(w, req)

			if w.Code == http.StatusTeapot {
				t.Fatalf("POST %s was intercepted by the catch-all ban route (got 418 Teapot)", tt.path)
			}
		})
	}
}

// TestRouting_FullLoginLifecycle exercises the complete spec Success Criteria
// lifecycle end-to-end through the real chi router, middleware, and handlers
// together: unauthenticated GET / redirects to /l0g1n, POST /l0g1n with valid
// credentials grants access, POST /logout revokes the session, and a
// subsequent GET / redirects to /l0g1n again.
func TestRouting_FullLoginLifecycle(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	ipRepo := repository.NewInMemoryIpRepository()
	sessionRepo := repository.NewInMemorySessionRepository(config.DefaultSessionTTL)
	conf := &config.Config{LoginUser: "alice", LoginPassHash: string(hash)}
	authHdlr := NewAuthHandler(sessionRepo, conf, testTemplate(t))
	pagesHdlr := NewPagesHandler(testTemplate(t))
	r := New(ipRepo, sessionRepo, authHdlr, pagesHdlr)

	// Step 1: unauthenticated GET / returns 404 (login page is not revealed).
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.Mux().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unauthenticated GET / status = %d, want %d", w.Code, http.StatusNotFound)
	}

	// Step 2: POST /l0g1n with correct credentials succeeds and sets a cookie.
	form := url.Values{"username": {"alice"}, "password": {"correct-password"}}
	loginReq := httptest.NewRequest(http.MethodPost, "/l0g1n", strings.NewReader(form.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginW := httptest.NewRecorder()
	r.Mux().ServeHTTP(loginW, loginReq)

	if loginW.Code != http.StatusSeeOther {
		t.Fatalf("POST /l0g1n status = %d, want %d", loginW.Code, http.StatusSeeOther)
	}
	cookies := loginW.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected exactly 1 cookie after login, got %d", len(cookies))
	}
	sessionCookie := cookies[0]

	// Step 3: a subsequent GET / with the session cookie succeeds.
	accessReq := httptest.NewRequest(http.MethodGet, "/", nil)
	accessReq.AddCookie(sessionCookie)
	accessW := httptest.NewRecorder()
	r.Mux().ServeHTTP(accessW, accessReq)

	if accessW.Code != http.StatusOK {
		t.Fatalf("authenticated GET / status = %d, want %d", accessW.Code, http.StatusOK)
	}

	// Step 4: POST /logout with the session cookie revokes the session,
	// clears the cookie with the same name/path, and redirects 303 to /l0g1n.
	logoutReq := httptest.NewRequest(http.MethodPost, "/logout", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	r.Mux().ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusSeeOther {
		t.Fatalf("POST /logout status = %d, want %d", logoutW.Code, http.StatusSeeOther)
	}
	if loc := logoutW.Header().Get("Location"); loc != "/l0g1n" {
		t.Fatalf("POST /logout Location = %q, want %q", loc, "/l0g1n")
	}
	logoutCookies := logoutW.Result().Cookies()
	if len(logoutCookies) != 1 {
		t.Fatalf("expected exactly 1 cookie on logout response, got %d", len(logoutCookies))
	}
	clearedCookie := logoutCookies[0]
	if clearedCookie.Name != sessionCookie.Name {
		t.Fatalf("cleared cookie name = %q, want %q (same name as login-set cookie)", clearedCookie.Name, sessionCookie.Name)
	}
	if clearedCookie.Path != sessionCookie.Path {
		t.Fatalf("cleared cookie path = %q, want %q (same path as login-set cookie)", clearedCookie.Path, sessionCookie.Path)
	}
	if clearedCookie.MaxAge != -1 {
		t.Fatalf("cleared cookie MaxAge = %d, want -1", clearedCookie.MaxAge)
	}

	// Step 5: GET / after logout returns 404 (revoked session is rejected server-side).
	reReq := httptest.NewRequest(http.MethodGet, "/", nil)
	reReq.AddCookie(sessionCookie)
	reW := httptest.NewRecorder()
	r.Mux().ServeHTTP(reW, reReq)

	if reW.Code != http.StatusNotFound {
		t.Fatalf("GET / after logout status = %d, want %d", reW.Code, http.StatusNotFound)
	}
}
