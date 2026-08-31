package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/grzegab/wok/internal/category"
	"github.com/grzegab/wok/internal/config"
	"github.com/grzegab/wok/internal/crawler"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/postgres"
)

// main is responsible for collecting HTML from job websites, it finds URL to collect
// and save job deatils into DB
func main() {
	log.Println("Starting fun ...")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// load config
	conf := config.Load()

	// check DB
	pgPool, err := postgres.NewClient(conf.PostgresDSN, false)
	if err != nil {
		log.Printf("Warning: could not connect to postgres: %v\n", err)
		panic("Database missing")
	} else {
		log.Println("Connected to Postgres")
		defer pgPool.Close()
	}
	db := database.New(pgPool)

	// start crawler
	wg := sync.WaitGroup{}

	cr := crawler.NewOfferFetcher(category.All)
	wg.Go(func() {
		cr.Fetch(ctx)
	})

	ot := crawler.NewOfferTransformer()
	wg.Go(func() {
		ot.Transform(ctx, cr.FetchedUrls)
	})

	op := crawler.NewOfferPersist(db)
	wg.Go(func() {
		op.Persist(ot.Offers)
	})

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)

	<-sig

	log.Println("Shutting down...")
	cancel()

	wg.Wait()

	log.Println("Done, bye bye!")
}
