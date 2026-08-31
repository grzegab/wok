#!/usr/bin/env bash
#
# Deploys the current main branch on the VPS: backs the database up, stops the
# stack, pulls the new code, rebuilds the images, applies the schema updates
# and starts everything again.
#
# Run it from anywhere: ./scripts/deploy.sh (or `make deploy`).
#
# Docker on the VPS needs root, so every docker call - including the ones the
# Makefile targets make - goes through $DOCKER. Export DOCKER=docker to drop
# the sudo (e.g. once the deploying user is in the docker group).

set -euo pipefail

cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-sudo docker}"
export DOCKER

step() {
    printf '\n==> %s\n' "$1"
}

step "Backing up the database"
if $DOCKER compose ps --status running --services | grep -qx postgres; then
    make db-backup
else
    echo "postgres is not running, skipping backup"
fi

step "Stopping the stack"
$DOCKER compose down

step "Pulling the new code"
git pull --ff-only

step "Rebuilding the images"
$DOCKER compose build

step "Starting postgres"
$DOCKER compose up -d postgres

echo "Waiting for postgres to accept connections..."
until $DOCKER compose exec -T postgres sh -c 'pg_isready -U "$POSTGRES_USER"' >/dev/null 2>&1; do
    sleep 1
done

# schema.sql only runs when the database is first initialised, so schema
# changes that must reach an existing database are applied here. Every
# statement has to be idempotent - this runs on every deploy. psql connects as
# POSTGRES_USER (wok), whose search_path resolves to the wok schema the app
# reads from; running this as any other role would create objects in public,
# where the app cannot see them.
step "Applying schema updates"
$DOCKER compose exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' <<'SQL'
CREATE INDEX IF NOT EXISTS possibilities_category_created_at_idx
    ON possibilities (category, created_at DESC, id DESC);

SELECT schemaname, indexname FROM pg_indexes WHERE tablename = 'possibilities';
SQL

step "Starting the rest of the stack"
$DOCKER compose up -d

$DOCKER compose ps

printf '\nDeploy done. Follow the logs with: %s compose logs -f\n' "$DOCKER"
