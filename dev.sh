#!/usr/bin/env bash
# GitCastle local development launcher.
#
# Starts everything needed to use the app:
#   1. Postgres (docker compose)
#   2. Go API server on :8080
#   3. Vite frontend with hot reload on :5173 (proxies /api to the API)
#
# Usage:
#   ./dev.sh              # start everything, tail logs (Ctrl-C stops all)
#   ./dev.sh --api-only   # backend only; frontend runs separately if wanted
#
# Idempotent: safe to re-run. Already-running pieces are left alone.
set -euo pipefail

cd "$(dirname "$0")"

API_PORT="${GITCASTLE_API_PORT:-8080}"
FRONTEND_PORT="${GITCASTLE_FRONTEND_PORT:-5173}"
REPO_ROOT_DIR="${REPOSITORY_ROOT:-./var/repositories}"
PIDS=()

log() { printf '\033[1;32m[gitcastle]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[gitcastle]\033[0m %s\n' "$*" >&2; exit 1; }

cleanup() {
  log "shutting down..."
  for pid in "${PIDS[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

require() { command -v "$1" >/dev/null 2>&1 || die "$1 is required but not installed"; }
require docker
require go

# --- 1. Database ------------------------------------------------------------
if ! docker info >/dev/null 2>&1; then
  log "starting Docker daemon..."
  open -a Docker 2>/dev/null || die "Docker daemon is not running -- start Docker Desktop first"
  for _ in $(seq 1 30); do
    docker info >/dev/null 2>&1 && break
    sleep 2
  done
  docker info >/dev/null 2>&1 || die "Docker did not become ready"
fi

if [ "$(docker compose ps --status running postgres 2>/dev/null | grep -c gitcastle || true)" -eq 0 ]; then
  log "starting Postgres..."
  docker compose up -d postgres
else
  log "Postgres already running"
fi

for _ in $(seq 1 30); do
  docker compose exec -T postgres pg_isready -U gitcastle >/dev/null 2>&1 && break
  sleep 1
done
docker compose exec -T postgres pg_isready -U gitcastle >/dev/null 2>&1 || die "Postgres never became ready"

# --- 2. API server ----------------------------------------------------------
mkdir -p "$REPO_ROOT_DIR"

export SECRET_ENCRYPTION_KEY="${SECRET_ENCRYPTION_KEY:-$(python3 -c 'import secrets; print(secrets.token_hex(32))')}"
export GITCASTLE_INTERNAL_TOKEN="${GITCASTLE_INTERNAL_TOKEN:-$(python3 -c 'import secrets; print(secrets.token_hex(24))')}"
export HTTP_ADDR=":${API_PORT}"
export REPOSITORY_ROOT="$REPO_ROOT_DIR"


# Keep Go caches inside the workspace: no dependency on a preconfigured
# GOCACHE/GOMODCACHE, and nothing written outside the checkout.
export GOCACHE="${GOCACHE:-$PWD/.gocache}"
export GOMODCACHE="${GOMODCACHE:-$PWD/.gomodcache}"
export GOPATH="${GOPATH:-$PWD/.gopath}"

log "building API..."
go build -o var/gitcastle ./cmd/gitcastle

if lsof -nP -iTCP:"$API_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
  log "port $API_PORT already in use -- assuming the API is already running"
else
  log "starting API on :$API_PORT..."
  ./var/gitcastle &
  PIDS+=("$!")
fi

for _ in $(seq 1 20); do
  curl -sf "http://localhost:${API_PORT}/healthz" >/dev/null 2>&1 && break
  sleep 0.5
done
curl -sf "http://localhost:${API_PORT}/healthz" >/dev/null 2>&1 || die "API failed to become healthy (check logs above)"

# --- 3. Frontend ------------------------------------------------------------
if [[ "${1:-}" != "--api-only" ]]; then
  require node
  export PATH="$PWD/.local-bin:$PATH"
  export COREPACK_HOME="$PWD/.corepack"
  command -v pnpm >/dev/null 2>&1 || die "pnpm not found (corepack shim missing? run: ls .local-bin)"

  (cd frontend && pnpm install --frozen-lockfile >/dev/null 2>&1)

  if lsof -nP -iTCP:"$FRONTEND_PORT" -sTCP:LISTEN >/dev/null 2>&1; then
    log "frontend already running on :$FRONTEND_PORT"
  else
    log "starting frontend on :$FRONTEND_PORT (hot reload)..."
    # Point the dev proxy at whichever port the API uses.
    GITCASTLE_API="http://localhost:${API_PORT}" pnpm --dir frontend exec vite --port "$FRONTEND_PORT" --strictPort &
    PIDS+=("$!")
  fi
fi

cat <<BANNER

  ----------------------------------------------
   GitCastle is up.

   App        http://localhost:${FRONTEND_PORT}
   API        http://localhost:${API_PORT}/healthz
   Git push   git push http://<user>:<pass>@localhost:${API_PORT}/git/<owner>/<repo>.git <branch>

   Press Ctrl-C to stop.
  ----------------------------------------------

BANNER

wait
