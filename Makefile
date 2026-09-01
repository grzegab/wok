.PHONY: up down build logs sqlc hashgen deploy db-backup db-dump db-restore db-reset

BACKUP_DIR := backups

# How docker is invoked. The VPS needs `make DOCKER="sudo docker" <target>`
# (or DOCKER exported in the shell) unless the user is in the docker group.
DOCKER ?= docker

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
