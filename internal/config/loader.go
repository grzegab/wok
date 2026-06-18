package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	RedisAddr string
	WebAddr   string
	SentryDsn string
}

func Load() *Config {
	log.Println("Reading config")

	// read .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("error loading .env file")
	}

	redisAddr := os.Getenv("REDIS_ADDR")
	webAddr := os.Getenv("WEB_ADDR")
	sentryDsn := os.Getenv("SENTRY_DSN")

	return &Config{
		RedisAddr: redisAddr,
		WebAddr:   webAddr,
		SentryDsn: sentryDsn,
	}
}
