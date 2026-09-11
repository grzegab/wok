package config

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const DefaultSessionTTL = 24 * time.Hour
const RequestTimeout = 10 * time.Second

type Config struct {
	RedisAddr      string
	PostgresDSN    string
	WebAddr        string
	WebDomain      string
	SentryDsn      string
	LoginUser      string
	LoginPassHash  string
	SessionTTL     time.Duration
	RequestTimeout time.Duration
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
	webDomain := os.Getenv("WEB_DOMAIN")
	sentryDsn := os.Getenv("SENTRY_DSN")
	loginUser := os.Getenv("LOGIN_USER")
	loginPassHash := os.Getenv("LOGIN_PASS_HASH")

	return &Config{
		RedisAddr:      redisAddr,
		PostgresDSN:    postgresDSN(),
		WebAddr:        webAddr,
		WebDomain:      webDomain,
		SentryDsn:      sentryDsn,
		LoginUser:      loginUser,
		LoginPassHash:  loginPassHash,
		SessionTTL:     DefaultSessionTTL,
		RequestTimeout: RequestTimeout,
	}
}

func postgresDSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"),
		os.Getenv("POSTGRES_PASSWORD"),
		os.Getenv("POSTGRES_HOST"),
		os.Getenv("POSTGRES_PORT"),
		os.Getenv("POSTGRES_DB"),
	)
}
