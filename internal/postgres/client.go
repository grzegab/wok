package postgres

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

func NewClient(dsn string, debug bool) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("could not parse postgres dsn: %w", err)
	}

	if debug {
		poolConfig.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   tracelog.LoggerFunc(logQuery),
			LogLevel: tracelog.LogLevelDebug,
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("could not connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("could not connect to postgres: %w", err)
	}

	return pool, nil
}

func logQuery(_ context.Context, _ tracelog.LogLevel, msg string, data map[string]any) {
	if args, ok := data["args"].([]any); ok {
		data["args"] = fmt.Sprintf("[REDACTED %d params]", len(args))
	}
	log.Printf("[postgres] %s %v", msg, data)
}
