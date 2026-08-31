# Project building instructions

Each `cmd/*` binary has its own Dockerfile under `docker/<service>/Dockerfile`, built from
the **project root** so it can access the whole module:

- `docker/web/Dockerfile` — the HTTP panel (`cmd/web`)
- `docker/crawler/Dockerfile` — the job-offer crawler (`cmd/crawler`)
- `docker/hashgen/Dockerfile` — one-off CLI to generate a bcrypt hash for `LOGIN_PASS_HASH` (`cmd/hashgen`)

## Running the full stack

`docker-compose.yaml` at the project root wires up `redis`, `postgres`, `web` and `crawler`.
Use the Makefile targets rather than calling `docker compose`/`docker build` directly:

```bash
make up      # build + start the whole stack in the background
make down    # stop the stack
make logs    # follow logs for all services
make build   # (re)build images without starting
```

Only the `web` service publishes a port to the host (`80`), for logging into the panel.
`redis` and `postgres` are reachable only from other containers on the compose network —
there is no way to reach them from outside the VPS.

## Deploying on the VPS

One command does the whole deploy — database backup, `docker compose down`, `git pull`,
image rebuild, schema updates and start:

```bash
make deploy      # or ./scripts/deploy.sh
```

The schema updates step exists because `schema.sql` is only executed when Postgres
initialises an empty data volume. Anything that has to reach an already-populated
database (a new index, a new column) belongs in the idempotent SQL block inside
`scripts/deploy.sh`, which runs on every deploy as the `wok` role.

## Generating a login password hash

`hashgen` isn't part of the always-on stack (it's an interactive one-off), so it's run directly:

```bash
make hashgen
```

## Database backup / restore / reset

Since Postgres isn't reachable from the host, these all run through `docker compose exec`
against the running `postgres` container:

```bash
make db-backup                          # dump to backups/wok_<timestamp>.sql
make db-restore FILE=backups/wok_20260101_120000.sql   # restore a dump
make db-reset                           # wipe all data, re-apply schema.sql
```

`backups/` lives at the project root (gitignored) and is created on first use.
`db-reset` drops and recreates the `public` schema, then re-runs
`internal/database/schema.sql` (already mounted into the container as its init script) —
this is a destructive operation, take a `make db-backup` first if you need the data.

## Technical details

Every Dockerfile follows the same two-stage pattern:
1. **Builder stage:** `golang:1.26-alpine`, compiles a static Linux binary (`CGO_ENABLED=0`).
2. **Final stage:** `alpine:latest` with just the binary (and, for the crawler, Chromium —
   it drives a headless browser via `chromedp`), running as a non-root user.

This keeps the final images small and reduces attack surface.
