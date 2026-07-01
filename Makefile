.PHONY: run up down build sqlc

# Start Redis + Postgres via Docker Compose, then run the Go app locally
run: up
	go run ./cmd/web

up:
	docker compose up -d

down:
	docker compose down

build:
	docker build -t card-landing -f docker/Dockerfile .

sqlc:
	sqlc generate
