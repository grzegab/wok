package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

const DefaultSessionTTL = 24 * time.Hour
const RequestTimeout = 10 * time.Second

type Config struct {
	RedisAddr      string
	WebAddr        string
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
	sentryDsn := os.Getenv("SENTRY_DSN")
	loginUser := os.Getenv("LOGIN_USER")
	loginPassHash := os.Getenv("LOGIN_PASS_HASH")

	return &Config{
		RedisAddr:      redisAddr,
		WebAddr:        webAddr,
		SentryDsn:      sentryDsn,
		LoginUser:      loginUser,
		LoginPassHash:  loginPassHash,
		SessionTTL:     DefaultSessionTTL,
		RequestTimeout: RequestTimeout,
	}
}
