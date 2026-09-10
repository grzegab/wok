package main

import (
	"context"
	"github.com/grzegab/wok/internal/nofluff"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/grzegab/wok/internal/category"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/justjoin"
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

	jjf := justjoin.NewOfferFetcher(category.JustJoinAll)
	wg.Go(func() {
		jjf.Fetch(ctx)
	})

	jjt := justjoin.NewOfferTransformer(db)
	wg.Go(func() {
		jjt.Transform(ctx, jjf.FetchedUrls)
	})

	jjp := justjoin.NewOfferPersist(db)
	wg.Go(func() {
		jjp.Persist(jjt.Offers)
	})

	nff := nofluff.NewNoFluffFetcher(category.NoFluffAll)
	wg.Go(func() {
		nff.FetchOffers(ctx)
	})

	nft := nofluff.NewOfferTransformer(db)
	wg.Go(func() {
		nft.Transform(ctx, nff.RawOfferBuff)
	})

	nfp := nofluff.NewOfferPersist(db)
	wg.Go(func() {
		nfp.Persist(nft.OffersDb)
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
