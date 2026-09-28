#!/usr/bin/env bash
# Local Postgres + Go/Air + Angular. Compatible with macOS Bash 3.2.
set -euo pipefail
set -m # Each background app gets a process group, including its children.

ROOT="$(cd "$(dirname "$0")" && pwd)"
export PATH="$HOME/go/bin:/opt/homebrew/bin:$PATH"
PIDS=""

die() { printf 'Error: %s\n' "$*" >&2; exit 1; }
compose() { docker compose --progress plain -p findvibe-local -f "$ROOT/compose.yaml" "$@"; }
prefix() {
  local line
  while IFS= read -r line || [ -n "$line" ]; do
    printf '[%s] %s\n' "$1" "$line"
  done
}

cleanup() {
  local status=$? pid
  trap - EXIT INT TERM
  printf '\nStopping FindVibe…\n'
  for pid in $PIDS; do kill -TERM -- "-$pid" 2>/dev/null || true; done
  if [ -n "$PIDS" ]; then
    sleep 1
    for pid in $PIDS; do kill -KILL -- "-$pid" 2>/dev/null || true; done
    wait 2>/dev/null || true
  fi
  compose down || status=1
  exit "$status"
}

for cmd in go air node npm docker curl lsof; do
  command -v "$cmd" >/dev/null 2>&1 || die "Missing $cmd; see README.md prerequisites."
done
docker compose version >/dev/null 2>&1 || die 'Docker Compose is missing.'
docker info >/dev/null 2>&1 || die 'Start Docker Desktop, then retry ./start.sh.'
for port in 8081 4200 55432; do
  if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
    die "Port $port is in use. Stop that service before starting FindVibe."
  fi
done

[ -f "$ROOT/backend/.env" ] || cp "$ROOT/backend/.env.example" "$ROOT/backend/.env"
if [ ! -d "$ROOT/frontend/node_modules" ]; then
  printf 'Installing frontend dependencies…\n'
  (cd "$ROOT/frontend" && npm ci --no-audit --no-fund)
fi

trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
compose up -d --wait --wait-timeout 60

# Explicit local settings prevent inherited production DB variables taking over.
(
  cd "$ROOT/backend"
  export IS_LOCAL=true PORT=8081
  export DATABASE_URL='postgres://postgres:postgres@127.0.0.1:55432/findvibe?sslmode=disable'
  air 2>&1 | prefix backend
) &
PIDS="$PIDS $!"
(
  cd "$ROOT/frontend"
  npm run dev -- --host 127.0.0.1 --port 4200 2>&1 | prefix frontend
) &
PIDS="$PIDS $!"

check_apps() {
  local pid
  for pid in $PIDS; do
    kill -0 "$pid" 2>/dev/null || die 'An app exited; see its logs above.'
  done
}

for ((attempt=0; attempt<120; attempt++)); do
  check_apps
  if curl -fsS --max-time 1 http://localhost:8081/health >/dev/null 2>&1 &&
     curl -fsS --max-time 1 http://localhost:4200/ >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
[ "$attempt" -lt 120 ] || die 'Startup timed out; see the app logs above.'
printf '\nFindVibe ready:\n  Frontend: http://localhost:4200\n  API:      http://localhost:8081/health\nCtrl-C stops both apps and Postgres; database data is kept.\n\n'
while :; do check_apps; sleep 1; done
