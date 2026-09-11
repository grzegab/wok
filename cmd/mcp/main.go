package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/grzegab/wok/internal/config"
	database "github.com/grzegab/wok/internal/database/generated"
	"github.com/grzegab/wok/internal/mcpserver"
	"github.com/grzegab/wok/internal/postgres"
)

// readOnlySession makes Postgres itself reject any write on this connection,
// independent of which queries the server happens to register.
const readOnlySession = "-c default_transaction_read_only=on"

// main serves the WOK database to an MCP client over stdin/stdout. The client
// runs it through ssh, so stdout carries JSON-RPC and nothing else - every
// diagnostic has to go to stderr.
func main() {
	log.SetOutput(os.Stderr)

	debug := flag.Bool("D", false, "enable debug mode (verbose logging, SQL tracing)")
	flag.Parse()

	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		log.Println("Debug mode enabled")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	conf := config.Load()

	pgPool, err := postgres.NewClient(conf.PostgresDSN+"&options="+url.QueryEscape(readOnlySession), *debug)
	if err != nil {
		log.Fatalf("could not connect to postgres: %v", err)
	}
	defer pgPool.Close()

	log.Println("Connected to Postgres, serving MCP over stdio")

	if err := mcpserver.New(database.New(pgPool)).Run(ctx, &mcp.StdioTransport{}); err != nil {
		log.Fatalf("mcp server stopped: %v", err)
	}

	log.Println("Client disconnected, bye bye!")
}
