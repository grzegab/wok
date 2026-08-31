package router

import (
	"context"
	"html/template"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grzegab/wok/internal/category"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/handlers"
	"github.com/grzegab/wok/internal/repository"
	"github.com/grzegab/wok/internal/templates"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// fakeApplicationStore is an in-memory app.ApplicationStore used to exercise
// the offers routes without a real database.
type fakeApplicationStore struct {
	apps map[int64]database.Application
}

func newFakeApplicationStore(apps ...database.Application) *fakeApplicationStore {
	s := &fakeApplicationStore{apps: make(map[int64]database.Application)}
	for _, a := range apps {
		s.apps[a.ID] = a
	}
	return s
}

func (s *fakeApplicationStore) CreateApplication(_ context.Context, arg database.CreateApplicationParams) (database.Application, error) {
	return database.Application{}, nil
}

func (s *fakeApplicationStore) GetApplication(_ context.Context, id int64) (database.Application, error) {
	a, ok := s.apps[id]
	if !ok {
		return database.Application{}, pgx.ErrNoRows
	}
	return a, nil
}

func (s *fakeApplicationStore) ListApplications(_ context.Context) ([]database.Application, error) {
	return nil, nil
}

func (s *fakeApplicationStore) UpdateApplication(_ context.Context, arg database.UpdateApplicationParams) (database.Application, error) {
	return database.Application{}, nil
}

func (s *fakeApplicationStore) DeleteApplication(_ context.Context, id int64) error {
	return nil
}

func testOffersTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := templates.Parse(templates.Offers)
	if err != nil {
		t.Fatalf("could not parse offers templates: %v", err)
	}
	return tmpl
}

func testDefaultsTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := templates.Parse(templates.Defaults)
	if err != nil {
		t.Fatalf("could not parse defaults templates: %v", err)
	}
	return tmpl
}

func testAuthTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := templates.Parse(templates.Auth)
	if err != nil {
		t.Fatalf("could not parse auth templates: %v", err)
	}
	return tmpl
}

func testStatsTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := templates.Parse(templates.Stats)
	if err != nil {
		t.Fatalf("could not parse stats templates: %v", err)
	}
	return tmpl
}

func testScrapedTemplate(t *testing.T) *template.Template {
	t.Helper()
	tmpl, err := templates.Parse(templates.Scraped)
	if err != nil {
		t.Fatalf("could not parse scraped templates: %v", err)
	}
	return tmpl
}

// fakePossibilityStatsStore is an in-memory handlers.PossibilityStatsStore
// used to exercise the stats route without a real database.
type fakePossibilityStatsStore struct{}

func (s *fakePossibilityStatsStore) StatsPossibilityByCategory(_ context.Context, _ string) ([]database.StatsPossibilityByCategoryRow, error) {
	return nil, nil
}

// fakePossibilityStore is an in-memory handlers.PossibilityStore used to
// exercise the scraped offers route without a real database.
type fakePossibilityStore struct{}

func (s *fakePossibilityStore) ListPossibilitiesByCategory(_ context.Context, _ string) ([]database.Possibility, error) {
	return nil, nil
}

func newTestRouter(t *testing.T, conf *config.Config, sessionRepo SessionRepository, store *fakeApplicationStore) *Router {
	t.Helper()

	ipRepo := repository.NewInMemoryIpRepository()
	authHdlr := handlers.NewAuthHandler(sessionRepo, conf, testAuthTemplate(t))
	defaultHdlr := handlers.NewDefaultHandler(testDefaultsTemplate(t))
	offersHdlr := handlers.NewOffersHandler(store, testOffersTemplate(t), 5*time.Second)
	statsHdlr := handlers.NewStatsHandler(&fakePossibilityStatsStore{}, testStatsTemplate(t), 5*time.Second)
	scrapedHdlr := handlers.NewScrapedHandler(&fakePossibilityStore{}, testScrapedTemplate(t), 5*time.Second)

	return New(ipRepo, sessionRepo, authHdlr, defaultHdlr, offersHdlr, statsHdlr, scrapedHdlr)
}

// TestRouting_LoginNotInterceptedByCatchAllBan guards the Chi routing
// precedence: the explicit POST /l0g1n registration must resolve ahead of
// the catch-all POST /* ban route.
func TestRouting_LoginNotInterceptedByCatchAllBan(t *testing.T) {
	sessionRepo := repository.NewInMemorySessionRepository(config.DefaultSessionTTL)
	r := newTestRouter(t, &config.Config{}, sessionRepo, newFakeApplicationStore())

	req := httptest.NewRequest(http.MethodPost, "/l0g1n", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	r.Mux().ServeHTTP(w, req)

	if w.Code == http.StatusTeapot {
		t.Fatalf("POST /l0g1n was intercepted by the catch-all ban route (got 418 Teapot)")
	}
}

// TestRouting_FullLoginLifecycle exercises unauthenticated GET / (404, login
// not revealed), POST /l0g1n with valid credentials granting access, and
// POST /l0g0ut revoking the session so a subsequent request is rejected.
func TestRouting_FullLoginLifecycle(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("bcrypt.GenerateFromPassword: %v", err)
	}
	sessionRepo := repository.NewInMemorySessionRepository(config.DefaultSessionTTL)
	conf := &config.Config{LoginUser: "alice", LoginPassHash: string(hash)}
	r := newTestRouter(t, conf, sessionRepo, newFakeApplicationStore())

	// Step 1: unauthenticated GET /l1st is not reachable without a session.
	req := httptest.NewRequest(http.MethodGet, "/l1st", nil)
	w := httptest.NewRecorder()
	r.Mux().ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("unauthenticated GET /l1st status = %d, want %d", w.Code, http.StatusNotFound)
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

	// Step 3: a subsequent GET /l1st with the session cookie succeeds.
	accessReq := httptest.NewRequest(http.MethodGet, "/l1st", nil)
	accessReq.AddCookie(sessionCookie)
	accessW := httptest.NewRecorder()
	r.Mux().ServeHTTP(accessW, accessReq)

	if accessW.Code != http.StatusOK {
		t.Fatalf("authenticated GET /l1st status = %d, want %d", accessW.Code, http.StatusOK)
	}

	// Step 4: POST /l0g0ut with the session cookie revokes the session
	// server-side (verified in step 5). Note: AuthHandler.Logout currently
	// calls http.Redirect with http.StatusNotFound instead of a 3xx status,
	// so the response is not actually followed as a redirect by clients.
	logoutReq := httptest.NewRequest(http.MethodPost, "/l0g0ut", nil)
	logoutReq.AddCookie(sessionCookie)
	logoutW := httptest.NewRecorder()
	r.Mux().ServeHTTP(logoutW, logoutReq)

	if logoutW.Code != http.StatusNotFound {
		t.Fatalf("POST /l0g0ut status = %d, want %d", logoutW.Code, http.StatusNotFound)
	}

	// Step 5: GET /l1st after logout is rejected again (revoked session).
	reReq := httptest.NewRequest(http.MethodGet, "/l1st", nil)
	reReq.AddCookie(sessionCookie)
	reW := httptest.NewRecorder()
	r.Mux().ServeHTTP(reW, reReq)

	if reW.Code != http.StatusNotFound {
		t.Fatalf("GET /l1st after logout status = %d, want %d", reW.Code, http.StatusNotFound)
	}
}

// TestRouting_OffersDetailAndEditRoutes covers the new /1nf0/{id} and
// /upd4te/{id} routes added in this refactor, both requiring a valid session.
func TestRouting_OffersDetailAndEditRoutes(t *testing.T) {
	sessionRepo := &fakeSessionRepository{valid: true}
	store := newFakeApplicationStore(database.Application{ID: 42, CompanyName: "Acme"})
	r := newTestRouter(t, &config.Config{}, sessionRepo, store)

	tests := []struct {
		name string
		path string
	}{
		{"details", "/1nf0/42"},
		{"edit form", "/upd4te/42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "valid-token"})
			w := httptest.NewRecorder()

			r.Mux().ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d (body: %s)", tt.path, w.Code, http.StatusOK, w.Body.String())
			}
		})
	}

	t.Run("unknown id returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/1nf0/999", nil)
		req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "valid-token"})
		w := httptest.NewRecorder()

		r.Mux().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("GET /1nf0/999 status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}

// TestRouting_CategoryPages covers the per-category scraped and stats routes:
// every menu category resolves, and an unknown slug is a 404.
func TestRouting_CategoryPages(t *testing.T) {
	sessionRepo := &fakeSessionRepository{valid: true}
	r := newTestRouter(t, &config.Config{}, sessionRepo, newFakeApplicationStore())

	for _, cat := range category.All {
		for _, path := range []string{"/scr4p3d/" + cat.Slug, "/st4ts/" + cat.Slug} {
			t.Run(path, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, path, nil)
				req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "valid-token"})
				w := httptest.NewRecorder()

				r.Mux().ServeHTTP(w, req)

				if w.Code != http.StatusOK {
					t.Fatalf("GET %s status = %d, want %d (body: %s)", path, w.Code, http.StatusOK, w.Body.String())
				}
			})
		}
	}

	t.Run("unknown category returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/scr4p3d/cobol", nil)
		req.AddCookie(&http.Cookie{Name: handlers.SessionCookieName, Value: "valid-token"})
		w := httptest.NewRecorder()

		r.Mux().ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("GET /scr4p3d/cobol status = %d, want %d", w.Code, http.StatusNotFound)
		}
	})
}
