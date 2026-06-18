## Infrastructure

### Multi-Stage Docker Build (confidence 92, sources: config + docs)
Docker images use a two-stage build: a `golang:alpine` builder stage compiling a static binary, and a slim `alpine:latest` runtime stage containing only the binary and required resources (templates, static files). Keeps the image small and reduces attack surface.

### Build From Project Root (confidence 88, source: docs)
`docker build` must be run from the project root, referencing `docker/Dockerfile`: `docker build -t card-landing -f docker/Dockerfile .`

### Non-Root Container User (confidence 85, source: config)
Containers run as a dedicated non-root user (`appuser`), with app files explicitly chowned before switching via the `USER` directive.

### Static Binary via CGO_ENABLED=0 (confidence 85, source: config)
Go binaries are compiled with `CGO_ENABLED=0 GOOS=linux` to produce statically-linked binaries compatible with the minimal alpine runtime image.

### IP Blocking for Production Access (confidence 82, source: docs)
Because the app is deployed publicly, IP-blocking middleware is a required security control (see internal/router for ban/check middleware).

### Go Toolchain Version Alignment (confidence 80, source: config)
The `go.mod` declared Go version must match the Docker builder base image tag, keeping local and containerized builds consistent.

### Pinned Module Versions (confidence 80, source: config)
Go module dependencies are pinned to exact semantic versions (no ranges), for reproducible builds.

### Dependency Caching via Layered COPY (confidence 80, source: config)
Dockerfile copies `go.mod`/`go.sum` and runs `go mod download` before copying the rest of the source, leveraging Docker layer caching.

### Environment Variable Naming Convention (confidence 80, source: config)
Env vars use `SCREAMING_SNAKE_CASE` with domain-prefixed grouping (`REDIS_*`, `WEB_*`), loaded via godotenv.

### Minimal Curated Dependency Set (confidence 75, source: config)
Favor a small set of focused libraries (Chi, go-redis, godotenv) over large frameworks, keeping the dependency surface minimal.

### OS Package Upgrade in Both Build Stages (confidence 75, source: config)
Both Docker stages run `apk update && apk upgrade --no-cache` before proceeding, to patch base image packages at build time.

### `.env.default` as Documented Template (confidence 75, source: config)
A `.env.default` file documents required environment variables with placeholder values, serving as the canonical config reference; the real `.env` holds actual secrets and is gitignored.

### Redis as Sole External Service Dependency (confidence 75, source: config)
`docker-compose.yaml` defines only a Redis service, indicating Redis is the project's primary external infra dependency for local development.
