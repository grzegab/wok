# Technology Stack

## Overview
This document describes the technology choices for WOK, a personal job-application tracking web app.

## Languages

### Go (1.26)
- **Usage**: ~100% of codebase
- **Rationale**: Single static binary, simple deployment, strong stdlib (net/http, html/template) for a small personal server app
- **Key Features Used**: `html/template` with embedded FS, goroutines for graceful shutdown signal handling

## Frameworks

### Frontend
None — server-side rendered HTML via Go's `html/template` (`.gohtml` files in `internal/templates/`). No JS framework or build pipeline.

### Backend
- **Chi v5.3.0** — lightweight HTTP router and middleware (RealIP, request timeout, logging, panic recovery, Sentry error handler)

### Testing
None configured yet — no test files present in the repository.

## Database

### Redis (v9.20.1 client)
- **Type**: Key-value store
- **ORM/Client**: `go-redis/redis/v9`
- **Rationale**: Used for IP-ban storage; an in-memory fallback implementation (`InMemoryIpRepository`) is used when Redis is unavailable, via a shared `IpRepository` interface

## Build Tools & Package Management
- Go Modules (go.mod / go.sum)
- Build command: `CGO_ENABLED=0 GOOS=linux go build -o main ./cmd/web`

## Infrastructure

### Containerization
- Docker: multi-stage build, Alpine base image, non-root `appuser`, static binary
- Docker Compose: runs a local Redis service (`redis:latest`, port 6379)

### CI/CD
None configured yet.

### Hosting
Not yet decided; README notes the app will be placed on production with public IP access, hence the IP-blocking middleware.

## Development Tools

### Linting & Formatting
None configured yet — relies on default `go fmt`.

### Type Checking
N/A (Go is statically typed by default).

## Key Dependencies
- `github.com/go-chi/chi/v5` v5.3.0 — HTTP routing/middleware
- `github.com/redis/go-redis/v9` v9.20.1 — Redis client
- `github.com/joho/godotenv` v1.5.1 — `.env` config loading
- Sentry SDK — imported in `cmd/web/main.go` for error tracking (not yet declared in go.mod — known gap)

## Version Management
Go Modules with semantic versioning in `go.mod`/`go.sum`.

## Migration Path
N/A — new project, no legacy stack to migrate from.

---
*Last Updated*: 2026-06-18
*Auto-detected*: Language, frameworks, dependencies, Docker setup, and architecture were detected from `go.mod`, `cmd/web/main.go`, `internal/`, and `docker/`. Vision and goals were provided by the user.
