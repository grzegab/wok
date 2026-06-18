package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grzegab/wok/internal/config"
	"github.com/grzegab/wok/internal/redis"
	"github.com/grzegab/wok/internal/repository"
	"github.com/grzegab/wok/internal/router"
)

func main() {
	// check for debug
	startDebug()

	// load config
	conf := config.Load()

	var ipRepo repository.IpRepository
	rdb, err := redis.NewClient(conf.RedisAddr)
	if err != nil {
		log.Printf("Warning: could not connect to redis: %v\n", err)
		fmt.Println("Warning: using in memory db")
		ipRepo = repository.NewInMemoryIpRepository()
	} else {
		log.Println("Connected to Redis at", conf.RedisAddr)
		defer rdb.Close()

		ipRepo = repository.NewRedisIpRepository(rdb)
	}

	// Init sentry (for logging errors on prod)
	err = sentry.Init(sentry.ClientOptions{
		Dsn: conf.SentryDsn,
	})
	if err != nil {
		log.Printf("sentry.Init: %s\n", err)
	}

	// start web server (simple)
	r := router.New(ipRepo)
	server := &http.Server{
		Addr:              conf.WebAddr,
		Handler:           r.Mux(),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
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
