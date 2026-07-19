#!/usr/bin/env bash
# shadow-db-lint.sh — apply migrations to an ephemeral Postgres container.
#
# Replaces `atlas migrate lint --dev-url …`. Spins up a throwaway Postgres
# instance, runs every migration in $1 against it, and tears down regardless
# of outcome. Exit code matches goose's exit code.
#
# Usage:  shadow-db-lint.sh <migrations-dir>
#
# Connectivity: neither the container's Docker bridge IP nor host port
# forwarding is universally reachable across sandboxed CI environments (one
# strategy hangs in some sandboxes, the other in others), so this script
# probes the container IP first with a short bounded timeout and falls back
# to host port forwarding (127.0.0.1:<published-port>) if that probe fails.

set -u
set -o pipefail

DIR="${1:-migrations}"

if [[ ! -d "$DIR" ]]; then
  echo "shadow-db-lint: directory not found: $DIR" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "shadow-db-lint: docker is required" >&2
  exit 2
fi

if ! command -v goose >/dev/null 2>&1; then
  echo "shadow-db-lint: goose is required (go install github.com/pressly/goose/v3/cmd/goose@latest)" >&2
  exit 2
fi

CNAME="goose-lint-$$-$(date +%s)"
trap 'docker rm -f "$CNAME" >/dev/null 2>&1 || true' EXIT

echo "shadow-db-lint: starting ephemeral Postgres ($CNAME)..."
if ! docker run -d --rm --name "$CNAME" \
       -e POSTGRES_PASSWORD=lint \
       -p 127.0.0.1::5432 \
       postgres:16 >/dev/null; then
  echo "shadow-db-lint: failed to start container" >&2
  exit 2
fi

# Resolve container IP (tried first below) as well as the host-forwarded
# port (fallback if the container IP isn't reachable from the host).
IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$CNAME")
if [[ -z "$IP" ]]; then
  echo "shadow-db-lint: could not resolve container IP" >&2
  exit 2
fi

echo "shadow-db-lint: waiting for Postgres at ${IP}:5432..."
for _ in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15; do
  if docker exec "$CNAME" pg_isready -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

if ! docker exec "$CNAME" pg_isready -U postgres >/dev/null 2>&1; then
  echo "shadow-db-lint: Postgres did not become ready in time" >&2
  exit 2
fi

# The readiness loop above only proves Postgres is up *inside* the
# container; it says nothing about whether the host can route to the
# container's Docker bridge IP. Probe that reachability directly, bounded
# by `timeout` so an unreachable IP fails fast instead of hanging until
# goose's own connection attempt times out.
if timeout 3 bash -c "exec 3<>/dev/tcp/${IP}/5432" 2>/dev/null; then
  echo "shadow-db-lint: using container IP (${IP})"
  URL="postgres://postgres:lint@${IP}:5432/postgres?sslmode=disable"
else
  HOST_PORT=$(docker port "$CNAME" 5432/tcp | head -n1 | sed -E 's/.*:([0-9]+)$/\1/')
  if [[ -z "$HOST_PORT" ]]; then
    echo "shadow-db-lint: container IP unreachable and could not determine published host port" >&2
    exit 2
  fi
  echo "shadow-db-lint: container IP unreachable, falling back to host port-forwarding (127.0.0.1:${HOST_PORT})"
  URL="postgres://postgres:lint@127.0.0.1:${HOST_PORT}/postgres?sslmode=disable"
fi

echo "shadow-db-lint: applying $DIR via goose..."
goose -dir "$DIR" postgres "$URL" up
RC=$?

if [[ $RC -eq 0 ]]; then
  echo "shadow-db-lint: ok"
fi

exit $RC
