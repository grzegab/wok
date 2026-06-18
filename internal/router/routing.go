package router

import (
	"fmt"
	"io/fs"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/grzegab/wok/internal/repository"
	"github.com/grzegab/wok/internal/templates"

	"github.com/go-chi/chi/v5/middleware"
)

type Router struct {
	chi    *chi.Mux
	ipRepo repository.IpRepository
}

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

func New(ipRepo repository.IpRepository) *Router {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(CheckIP(ipRepo))
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(SentryHandler())

	routerObj := &Router{
		chi:    r,
		ipRepo: ipRepo,
	}

	// Assets
	subFS, _ := fs.Sub(templates.FSa, "assets")
	fsa := http.FileServer(http.FS(subFS))
	r.Handle("/assets/grzegab.jpg", http.StripPrefix("/assets/", fsa))
	r.Handle("/assets/favicon.ico", http.StripPrefix("/assets/", fsa))

	// Proper paths
	r.Get("/", landing)
	r.Get("/robots.txt", robotsTxt)

	// Ban most important GET
	var reason string
	for _, uri := range banUri {
		reason = fmt.Sprintf("banning %s GET", uri)
		r.Get(uri, func(w http.ResponseWriter, r *http.Request) {
			ban(routerObj.ipRepo, reason, w, r)
		})
	}

	// Ban other methods
	r.Post("/*", func(w http.ResponseWriter, r *http.Request) {
		reason = fmt.Sprintf("banning default POST: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})
	r.Put("/*", func(w http.ResponseWriter, r *http.Request) {
		reason = fmt.Sprintf("banning default PUT: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})
	r.Delete("/*", func(w http.ResponseWriter, r *http.Request) {
		reason = fmt.Sprintf("banning default Delete: %s", r.RequestURI)
		ban(routerObj.ipRepo, reason, w, r)
	})

	return routerObj
}

func (r *Router) Mux() chi.Router {
	return r.chi
}
