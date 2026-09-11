.PHONY: up down build logs sqlc hashgen deploy db-backup db-dump db-restore db-reset crawler mcp mcp-deploy cleaner cleaner-deploy

BACKUP_DIR := backups

# How docker is invoked. The VPS needs `make DOCKER="sudo docker" <target>`
# (or DOCKER exported in the shell) unless the user is in the docker group.
DOCKER ?= docker

# Where the MCP server binary is shipped. Override per host:
# make mcp-deploy WOK_SSH_HOST=root@1.2.3.4
WOK_SSH_HOST   ?= wok-vps
WOK_REMOTE_DIR ?= /opt/wok

# Build and start the whole stack (web, crawler, redis, postgres) in Docker
up:
	$(DOCKER) compose up -d --build

down:
	$(DOCKER) compose down

# (Re)build all service images without starting them
build:
	$(DOCKER) compose build

logs:
	$(DOCKER) compose logs -f

sqlc:
	sqlc generate

# Build and run the password-hash CLI interactively (not part of the always-on stack)
hashgen:
	$(DOCKER) build -t wok-hashgen -f docker/hashgen/Dockerfile .
	$(DOCKER) run --rm -it wok-hashgen

# Full deploy on the VPS: backup, stop, git pull, rebuild, apply schema updates, start
deploy:
	./scripts/deploy.sh

# Dump the Postgres database to a timestamped SQL file in backups/
# --clean --if-exists makes the dump self-contained (drops objects before
# recreating them), so db-restore is safe to run against a live database too.
db-backup:
	mkdir -p $(BACKUP_DIR)
	$(DOCKER) compose exec -T postgres sh -c 'pg_dump --clean --if-exists -U "$$POSTGRES_USER" "$$POSTGRES_DB"' > $(BACKUP_DIR)/wok_$$(date +%Y%m%d_%H%M%S).sql
	@echo "Backup written to $(BACKUP_DIR)/"

# Print a dump of the database to stdout for copy-pasting elsewhere.
# Structure only: make db-dump ARGS=--schema-only
db-dump:
	./scripts/db-dump.sh $(ARGS)

# Restore a SQL dump into Postgres: make db-restore FILE=backups/wok_20260101_120000.sql
db-restore:
	@test -n "$(FILE)" || { echo "Usage: make db-restore FILE=$(BACKUP_DIR)/<file>.sql"; exit 1; }
	$(DOCKER) compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" "$$POSTGRES_DB"' < $(FILE)

# Wipe all data and re-apply schema.sql, resetting the database to its initial (empty) state
db-reset:
	$(DOCKER) compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"'
	$(DOCKER) compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f /docker-entrypoint-initdb.d/schema.sql'

crawler:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/crawler ./cmd/crawler

# Cross-compile the MCP server that serves the database to an MCP client over ssh
mcp:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/wok-mcp ./cmd/mcp

# Ship it to the VPS. It is not part of the Compose stack: the client starts it
# per session over ssh, so a plain scp is the whole deploy.
mcp-deploy: mcp
	scp bin/wok-mcp $(WOK_SSH_HOST):$(WOK_REMOTE_DIR)/bin/wok-mcp

# Clean the database once: repair JSON location/tech and normalize technology
# names. Runs on the VPS host, from the binary cleaner-deploy shipped there;
# Postgres is on the loopback because .env names the Compose service, which the
# host cannot resolve. Safe to repeat, so cron can run it: cd /opt/wok && make cleaner
cleaner:
	POSTGRES_HOST=127.0.0.1 ./bin/wok-cleaner

# Cross-compile the cleaner and ship it to the VPS (run locally, like mcp-deploy)
cleaner-deploy:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/wok-cleaner ./cmd/cleaner
