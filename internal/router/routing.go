package router

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/grzegab/wok/internal/app"
	"github.com/grzegab/wok/internal/templates"
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
	app.BannedIps
	ipExists
}

type SessionRepository interface {
	app.AuthSessionStore
	sessionValidator
}

func New(ipRepo IpRepository, sessionRepo SessionRepository, authHdlr *app.AuthHandler, defaultHndl *app.DefaultHandler, offers app.IOffersHandler) *Router {
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

	// Assets: embedded static files (Bootstrap CSS, etc.) served privately so
	// they will not be visible on attacks
	assetsFS, err := fs.Sub(templates.FSa, "assets")
	if err != nil {
		panic(fmt.Sprintf("could not open embedded assets: %v", err))
	}

	// Public auth routes: must stay outside the RequireLogin group below so
	// unauthenticated visitors can reach them, and outside any per-path
	// exemption list so the ban routes further down stay reachable too.
	r.Get("/l0g1n", authHdlr.Login)
	r.Post("/l0g1n", authHdlr.Login)
	r.Get("/", defaultHndl.Landing)

	// Protected routes: only these are gated by RequireLogin, so the ban
	// catch-all routes below never get shadowed by a 404 from the auth check.
	r.Group(func(pr chi.Router) {
		pr.Use(RequireLogin(sessionRepo))
		pr.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServerFS(assetsFS)))
		pr.Post("/l0g0ut", authHdlr.Logout)
		pr.Get("/l1st", offers.List)
		pr.Get("/4dd", offers.ShowAdd)
		pr.Post("/4dd", offers.Add)
		//pr.Get("application/{id}", application_details)
	})

	// Ban most important GET
	for _, uri := range banUri {
		reason := fmt.Sprintf("banning %s GET", uri)
		r.Get(uri, func(w http.ResponseWriter, r *http.Request) {
			app.Ban(routerObj.ipRepo, reason, w, r)
		})
	}

	// Ban other methods
	r.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default POST: %s", r.RequestURI)
		app.Ban(routerObj.ipRepo, reason, w, r)
	})
	r.Put("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default PUT: %s", r.RequestURI)
		app.Ban(routerObj.ipRepo, reason, w, r)
	})
	r.Delete("/*", func(w http.ResponseWriter, r *http.Request) {
		reason := fmt.Sprintf("banning default Delete: %s", r.RequestURI)
		app.Ban(routerObj.ipRepo, reason, w, r)
	})

	return routerObj
}

func (r *Router) Mux() chi.Router {
	return r.chi
}
