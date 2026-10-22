# Project building instructions

Each `cmd/*` binary has its own Dockerfile under `docker/<service>/Dockerfile`, built from
the **project root** so it can access the whole module:

- `docker/web/Dockerfile` — the HTTP panel (`cmd/web`)
- `docker/crawler/Dockerfile` — the job-offer crawler (`cmd/crawler`)
- `docker/hashgen/Dockerfile` — one-off CLI to generate a bcrypt hash for `LOGIN_PASS_HASH` (`cmd/hashgen`)
- `docker/cleaner/Dockerfile` — one-off database cleaner (`cmd/cleaner`)
- `docker/mcp/Dockerfile` — MCP server an MCP client drives over ssh (`cmd/mcp`)

## Running the full stack

`docker-compose.yaml` at the project root wires up `redis`, `postgres`, `web` and `crawler`.
`cleaner` and `mcp` are in it too, under the `tools` profile: `make up` leaves them out, and
each run is a throwaway `docker compose run --rm -T <service>` container (`make cleaner`,
the MCP client's ssh command). `make build` and the deploy build their images as well.
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

Every docker call in the Makefile goes through `$(DOCKER)`, which defaults to plain
`docker` for local use. `scripts/deploy.sh` sets and exports `DOCKER="sudo docker"`, so the
targets it calls (`make db-backup`) run as root too; export `DOCKER=docker` before the
deploy once the deploying user is in the `docker` group. To run a single target as root by
hand: `make DOCKER="sudo docker" logs`.

The schema updates step exists because `schema.sql` is only executed when Postgres
initialises an empty data volume. Anything that has to reach an already-populated
database (a new index, a new column) belongs in the idempotent SQL block inside
`scripts/deploy.sh`, which runs on every deploy as the `wok` role.

## Moving to a new VPS

Everything stateful travels in one archive made by `scripts/migrate-export.sh`: the
Postgres dump (applications and scraped offers, so the crawler keeps its history and
dedupes against it), the `web_certs` volume and `.env`. Redis holds only sessions and IP
bans and is not moved - you log in again on the new server.

1. Prepare the new server: Docker Engine with the compose plugin, `git`, an ssh key that
   can pull the repo, then `git clone git@github.com:grzegab/wok.git ~/wok`. Keep the
   directory name `wok` (volume names derive from it) or export `COMPOSE_PROJECT_NAME=wok`.
2. Rehearse while the old server keeps running:
   ```bash
   # old server
   ./scripts/migrate-export.sh
   scp -P <port> wok-migration-*.tar.gz <user>@<new-host>:~/
   # new server
   ./scripts/migrate-import.sh ~/wok-migration-<timestamp>.tar.gz
   ```
   The import restores the dump, compares row counts with the export, then runs
   `scripts/deploy.sh`. Check the panel and `/st4ts/go` on the new server.
3. Cut over: run `./scripts/migrate-export.sh --final` on the old server (it stops the
   crawler and web before dumping, so nothing scraped later is lost), copy the archive and
   run the import again on the new server. The images are already built by the rehearsal,
   so the crawler is down only for the dump, copy and restore.
4. Point everything at the new host: the MCP client's ssh command, the SSH node of the n8n
   `ofertyPracyDaily` workflow, DNS if `WEB_DOMAIN` is set. Leave the old stack stopped
   (`make up` brings it back if something is wrong) until the old server expires.

The archive contains `.env` with the login hash and database password: it is created
`chmod 600`, and should be deleted from both servers after the move.

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
1. **Builder stage:** `golang:1.27.0-alpine`, compiles a static Linux binary (`CGO_ENABLED=0`).
2. **Final stage:** `alpine:latest` with just the binary (and, for the crawler, Chromium —
   it drives a headless browser via `chromedp`), running as a non-root user.

This keeps the final images small and reduces attack surface.
