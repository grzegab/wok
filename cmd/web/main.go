package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/handlers"
	"github.com/grzegab/wok/internal/postgres"
	"github.com/grzegab/wok/internal/redis"
	"github.com/grzegab/wok/internal/repository"
	"github.com/grzegab/wok/internal/router"
	"github.com/grzegab/wok/internal/templates"
	"golang.org/x/crypto/acme/autocert"
)

const (
	certCacheDir     = "certs"
	loginMaxAttempts = 5
	loginWindow      = 15 * time.Minute
)

func main() {
	debug := flag.Bool("D", false, "enable debug mode (verbose logging, SQL and Redis command tracing)")
	flag.Parse()

	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	// load config
	conf := config.Load()

	var ipRepo router.IpRepository
	rdb, err := redis.NewClient(conf.RedisAddr, *debug)
	if err != nil {
		log.Printf("Warning: could not connect to redis: %v\n", err)
		fmt.Println("Warning: using in memory db")
		ipRepo = repository.NewInMemoryIpRepository()
	} else {
		log.Println("Connected to Redis at", conf.RedisAddr)
		defer rdb.Close()

		ipRepo = repository.NewRedisIpRepository(rdb)
	}

	var sessionRepo router.SessionRepository
	if rdb != nil {
		sessionRepo = repository.NewRedisSessionRepository(rdb, conf.SessionTTL)
	} else {
		log.Printf("Warning: could not connect to redis: %v\n", err)
		fmt.Println("Warning: using in memory db")
		sessionRepo = repository.NewInMemorySessionRepository(conf.SessionTTL)
	}

	pgPool, err := postgres.NewClient(conf.PostgresDSN, *debug)
	if err != nil {
		log.Printf("Warning: could not connect to postgres: %v\n", err)
		panic("Database missing")
	} else {
		log.Println("Connected to Postgres")
		defer pgPool.Close()
	}

	offersTmpl, err := templates.Parse(templates.Offers)
	if err != nil {
		log.Printf("could not parse offers templates: %v", err)
		panic("Templates missing")
	}

	defaultsTmpl, err := templates.Parse(templates.Defaults)
	if err != nil {
		log.Printf("could not parse defaults templates: %v", err)
		panic("Templates missing")
	}

	authTmpl, err := templates.Parse(templates.Auth)
	if err != nil {
		log.Printf("could not parse auth templates: %v", err)
		panic("Templates missing")
	}

	statsTmpl, err := templates.Parse(templates.Stats)
	if err != nil {
		log.Printf("could not parse stats templates: %v", err)
		panic("Templates missing")
	}

	scrapedTmpl, err := templates.Parse(templates.Scraped)
	if err != nil {
		log.Printf("could not parse scraped templates: %v", err)
		panic("Templates missing")
	}

	// Init sentry (for logging errors on prod)
	err = sentry.Init(sentry.ClientOptions{
		Dsn: conf.SentryDsn,
	})
	if err != nil {
		log.Printf("sentry.Init: %s\n", err)
	}

	db := database.New(pgPool)

	var handlerTimeout = 5 * time.Second

	loginLimiter := repository.NewInMemoryLoginLimiter(loginMaxAttempts, loginWindow)
	authHandler := handlers.NewAuthHandler(sessionRepo, loginLimiter, conf, authTmpl)
	defaultHandler := handlers.NewDefaultHandler(defaultsTmpl)
	offerHandler := handlers.NewOffersHandler(db, offersTmpl, handlerTimeout)
	statsHandler := handlers.NewStatsHandler(db, statsTmpl, handlerTimeout)
	scrapedHandler := handlers.NewScrapedHandler(db, scrapedTmpl, handlerTimeout)
	r := router.New(ipRepo, sessionRepo, authHandler, defaultHandler, offerHandler, statsHandler, scrapedHandler)

	var servers []*http.Server
	if conf.WebDomain == "" {
		server := newServer(conf.WebAddr, r.Mux(), conf.RequestTimeout)
		servers = append(servers, server)
		go serve(server.ListenAndServe)
	} else {
		certManager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(conf.WebDomain),
			Cache:      autocert.DirCache(certCacheDir),
		}

		httpsServer := newServer(":443", r.Mux(), conf.RequestTimeout)
		httpsServer.TLSConfig = certManager.TLSConfig()
		httpServer := newServer(":80", certManager.HTTPHandler(nil), conf.RequestTimeout)
		servers = append(servers, httpsServer, httpServer)

		log.Println("Serving HTTPS for", conf.WebDomain)
		go serve(func() error { return httpsServer.ListenAndServeTLS("", "") })
		go serve(httpServer.ListenAndServe)
	}

	// shutdown if needed
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Println("App is running now. Press Ctrl+C to stop it.")
	<-sigCh
	fmt.Print("\n\n")
	log.Println("Shutdown signal received, shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, server := range servers {
		if err := server.Shutdown(ctx); err != nil {
			log.Println("error while shutdown:", err)
			panic("error while shutdown")
		}
	}

	fmt.Println("Bye, bye!")
}

func newServer(addr string, handler http.Handler, timeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadTimeout:       timeout,
		ReadHeaderTimeout: timeout,
		WriteTimeout:      timeout,
		IdleTimeout:       timeout,
	}
}

func serve(listen func() error) {
	if err := listen(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("server error: %v", err)
		panic("Server error")
	}
}
