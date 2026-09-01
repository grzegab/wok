#!/usr/bin/env bash
#
# Prints a dump of the Postgres database to stdout, so it can be copied and
# pasted elsewhere. Pass --schema-only for the structure without any rows.
#
# Run it from anywhere: ./scripts/db-dump.sh (or `make db-dump`).
#
# Docker on the VPS needs root, so the docker call goes through $DOCKER.
# Export DOCKER=docker to drop the sudo (e.g. once the user is in the docker
# group).

set -euo pipefail

cd "$(dirname "$0")/.."

DOCKER="${DOCKER:-sudo docker}"

# --clean --if-exists makes the dump self-contained: it drops the objects
# before recreating them, so it can be replayed against a live database.
DUMP_ARGS=(--clean --if-exists)

case "${1:-}" in
    --schema-only) DUMP_ARGS+=(--schema-only) ;;
    "") ;;
    *) echo "Usage: $0 [--schema-only]" >&2; exit 1 ;;
esac

$DOCKER compose exec -T postgres \
    sh -c "pg_dump ${DUMP_ARGS[*]} -U \"\$POSTGRES_USER\" \"\$POSTGRES_DB\""
