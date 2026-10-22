#!/usr/bin/env bash
#
# Packs everything the stack needs on a new server into one archive: the
# database dump, the TLS certificates volume and .env. Run it on the old VPS:
#
#   ./scripts/migrate-export.sh            # rehearsal, the stack keeps running
#   ./scripts/migrate-export.sh --final    # cutover, stops crawler and web first
#
# --final stops the writers so no offer scraped after the dump is lost; the old
# server stays stopped afterwards and can be restarted with `make up`.

set -euo pipefail

cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-sudo docker}"
PROJECT="${COMPOSE_PROJECT_NAME:-$(basename "$PWD")}"
FINAL=false
[[ "${1:-}" == "--final" ]] && FINAL=true

OUT="$PWD/wok-migration-$(date +%Y%m%d_%H%M%S).tar.gz"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

step() {
    printf '\n==> %s\n' "$1"
}

if $FINAL; then
    step "Stopping crawler and web"
    $DOCKER compose stop crawler web
fi

step "Dumping the database"
$DOCKER compose exec -T postgres sh -c 'pg_dump --clean --if-exists -U "$POSTGRES_USER" "$POSTGRES_DB"' > "$WORK/wok.sql"

step "Counting rows to compare after the import"
$DOCKER compose exec -T postgres sh -c 'psql -At -U "$POSTGRES_USER" "$POSTGRES_DB"' <<'SQL' | tee "$WORK/counts.txt"
SELECT 'applications', count(*), max(id) FROM applications;
SELECT 'possibilities', count(*), max(id) FROM possibilities;
SQL

step "Saving the TLS certificates volume"
$DOCKER run --rm -v "${PROJECT}_web_certs:/certs:ro" alpine tar -C /certs -cf - . > "$WORK/web_certs.tar"

cp .env "$WORK/.env"

tar -C "$WORK" -czf "$OUT" .
chmod 600 "$OUT"

printf '\nArchive: %s\n' "$OUT"
printf 'Copy it to the new server, e.g.: scp -P <port> %s <user>@<new-host>:~/\n' "$(basename "$OUT")"
