package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(addr string, debug bool) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	if debug {
		rdb.AddHook(debugHook{})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("could not connect to redis: %w", err)
	}

	return rdb, nil
}

type debugHook struct{}

var _ redis.Hook = debugHook{}

func (debugHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (debugHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		log.Printf("[redis] %s (%s)", redactArgs(cmd.Args()), time.Since(start))
		return err
	}
}

func (debugHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		for _, cmd := range cmds {
			log.Printf("[redis] %s (pipeline, %s)", redactArgs(cmd.Args()), time.Since(start))
		}
		return err
	}
}

// redactArgs keeps the command name (e.g. "set", "auth") but hides the
// arguments that follow it, since they may contain session tokens, IP-ban
// data, or other sensitive values.
func redactArgs(args []any) string {
	if len(args) == 0 {
		return ""
	}
	return fmt.Sprintf("%v [REDACTED %d args]", args[0], len(args)-1)
}
