package router

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var banUri = []string{
	"/.env",
	"/login",
	"/wp-*",
	"//wp-*",
	"/uploads*",
	"/public*",
	"/modules*",
	"/wordpress*",
	"/cgi-bin*",
	"//blog*",
	"//web*",
	"//website*",
	"/xmlrpc*",
	"//xmlrpc*",
	"/shop*",
	"//shop*",
	"/site*",
	"//site*",
	"/cms*",
	"//cms*",
	"/wp2*",
	"//wp2*",
	"/media*",
	"//media*",
	"/test*",
	"//test*",
}

type Router struct {
	chi         *chi.Mux
	ipRepo      IpRepository
	sessionRepo SessionRepository
}

type IpRepository interface {
	bannedIps
	ipExists
}

type SessionRepository interface {
	authSessionStore
	sessionValidator
}

func New(ipRepo IpRepository, sessionRepo SessionRepository, authHdlr *AuthHandler, pagesHdlr *PagesHandler) *Router {
	r := chi.NewRouter()
	r.Use(middleware.ClientIPFromRemoteAddr)
	r.Use(CheckIP(ipRepo))
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(SentryHandler())

	routerObj := &Router{
		chi:         r,
		ipRepo:      ipRepo,
		sessionRepo: sessionRepo,
	}

	// Assets

	// Public auth routes: must stay outside the RequireLogin group below so
	// unauthenticated visitors can reach them, and outside any per-path
	// exemption list so the ban routes further down stay reachable too.
	r.Get("/l0g1n", authHdlr.login)
	r.Post("/l0g1n", authHdlr.login)
	r.Post("/logout", authHdlr.logout)

	// Protected routes: only these are gated by RequireLogin, so the ban
	// catch-all routes below never get shadowed by a 404 from the auth check.
	r.Group(func(pr chi.Router) {
		pr.Use(RequireLogin(sessionRepo))
		pr.Get("/", pagesHdlr.landing)
		pr.Get("/list", pagesHdlr.list)
		//pr.Get("application/{id}", application_details)
	})

	// Ban most important GET
	for _, uri := range banUri {
		reason := fmt.Sprintf("banning %s GET", uri)
		r.Get(uri, func(w http.ResponseWriter, r *http.Request) {
			ban(routerObj.ipRepo, reason, w, r)
		})
	}

	// Ban other methods
	r.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default POST: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})
	r.Put("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default PUT: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})
	r.Delete("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default Delete: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})

	return routerObj
}

func (r *Router) Mux() chi.Router {
	return r.chi
}
