package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/grzegab/wok/internal/cleaner"
	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/postgres"
)

// main tidies up the database once and exits: it repairs possibilities whose
// location or tech were stored as raw JSON and normalizes technology names.
func main() {
	debug := flag.Bool("D", false, "enable debug mode (verbose logging, SQL tracing)")
	flag.Parse()

	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conf := config.Load()

	pgPool, err := postgres.NewClient(conf.PostgresDSN, *debug)
	if err != nil {
		log.Fatalf("could not connect to postgres: %v", err)
	}
	defer pgPool.Close()

	log.Println("Connected to Postgres")

	c := cleaner.NewCleaner(database.New(pgPool))

	// JSON is decoded first, so the lists it turns into get normalized too.
	fixed, err := c.FixJsonFields(ctx)
	log.Printf("Fixed %d possibilities stored with JSON location or tech\n", fixed)
	if err != nil {
		log.Fatalf("cleaning stopped: %v", err)
	}

	normalized, err := c.NormalizeTech(ctx)
	log.Printf("Normalized tech of %d possibilities\n", normalized)
	if err != nil {
		log.Fatalf("cleaning stopped: %v", err)
	}

	log.Println("Done, bye bye!")
}
