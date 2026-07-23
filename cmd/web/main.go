package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"html/template"
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

	offersTmpl, err := template.ParseFS(templates.Offers, "*/*.gohtml")
	if err != nil {
		log.Printf("could not parse offers templates: %v", err)
		panic("Templates missing")
	}

	defaultsTmpl, err := template.ParseFS(templates.Defaults, "*/*.gohtml")
	if err != nil {
		log.Printf("could not parse defaults templates: %v", err)
		panic("Templates missing")
	}

	authTmpl, err := template.ParseFS(templates.Auth, "*/*.gohtml")
	if err != nil {
		log.Printf("could not parse auth templates: %v", err)
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

	authHandler := handlers.NewAuthHandler(sessionRepo, conf, authTmpl)
	defaultHandler := handlers.NewDefaultHandler(defaultsTmpl)
	offerHandler := handlers.NewOffersHandler(db, offersTmpl, handlerTimeout)
	r := router.New(ipRepo, sessionRepo, authHandler, defaultHandler, offerHandler)

	server := &http.Server{
		Addr:              conf.WebAddr,
		Handler:           r.Mux(),
		ReadTimeout:       conf.RequestTimeout,
		ReadHeaderTimeout: conf.RequestTimeout,
		WriteTimeout:      conf.RequestTimeout,
		IdleTimeout:       conf.RequestTimeout,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			panic("Server error")
		}
	}()

	// shutdown if needed
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	fmt.Println("App is running now. Press Ctrl+C to stop it.")
	<-sigCh
	fmt.Print("\n\n")
	log.Println("Shutdown signal received, shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Println("error while shutdown:", err)
		panic("error while shutdown")
	}

	fmt.Println("Bye, bye!")
}
