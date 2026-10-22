#!/usr/bin/env bash
#
# Restores an archive made by migrate-export.sh on a new server and starts the
# stack. Run it from a fresh clone of the repo:
#
#   ./scripts/migrate-import.sh ~/wok-migration-<timestamp>.tar.gz
#
# It replaces the database of this server with the dump, so on a server that
# already holds data take a `make db-backup` first.

set -euo pipefail

cd "$(dirname "$0")/.."

ARCHIVE="${1:?usage: $0 <wok-migration-archive.tar.gz>}"
DOCKER="${DOCKER:-sudo docker}"
export DOCKER
PROJECT="${COMPOSE_PROJECT_NAME:-$(basename "$PWD")}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

step() {
    printf '\n==> %s\n' "$1"
}

tar -C "$WORK" -xzf "$ARCHIVE"

step "Installing .env"
if [[ -f .env ]] && ! cmp -s .env "$WORK/.env"; then
    cp .env ".env.before-import.$(date +%Y%m%d_%H%M%S)"
    echo "existing .env differs, kept a copy next to it"
fi
install -m 600 "$WORK/.env" .env

step "Starting postgres"
$DOCKER compose up -d postgres
# On a fresh volume the image first runs a temporary server for schema.sql that
# listens on the unix socket only, so wait for TCP to reach the real one.
until $DOCKER compose exec -T postgres sh -c 'pg_isready -h 127.0.0.1 -U "$POSTGRES_USER"' >/dev/null 2>&1; do
    sleep 1
done

step "Restoring the database"
$DOCKER compose exec -T postgres sh -c 'psql -q -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" "$POSTGRES_DB"' < "$WORK/wok.sql" >/dev/null

step "Restoring the TLS certificates volume"
$DOCKER volume create "${PROJECT}_web_certs" >/dev/null
$DOCKER run --rm -i -v "${PROJECT}_web_certs:/certs" alpine tar -C /certs -xf - < "$WORK/web_certs.tar"

step "Comparing row counts with the old server"
$DOCKER compose exec -T postgres sh -c 'psql -At -U "$POSTGRES_USER" "$POSTGRES_DB"' <<'SQL' > "$WORK/counts.new.txt"
SELECT 'applications', count(*), max(id) FROM applications;
SELECT 'possibilities', count(*), max(id) FROM possibilities;
SQL
if diff "$WORK/counts.txt" "$WORK/counts.new.txt"; then
    cat "$WORK/counts.new.txt"
    echo "row counts match"
else
    echo "row counts differ from the export, stopping before the stack starts" >&2
    exit 1
fi

step "Building and starting the stack"
./scripts/deploy.sh
