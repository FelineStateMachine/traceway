#!/usr/bin/env bash
#
# Runs the CLI smoke suite against a real, ephemeral Traceway backend.
#
# Spins up the embedded backend (SQLite, no external infra) seeded with one
# user + one project, discovers the seeded project id through the CLI itself,
# then runs `go test -tags smoke ./test/smoke/...` against it. This is the same
# orchestration the CLI GitHub Actions workflow runs — keep the logic here so it
# can be exercised on a laptop before pushing.
#
#   cli/scripts/smoke-ci.sh            # full run
#   cli/scripts/smoke-ci.sh --dry-run  # validate toolchain + layout only
#
# Env overrides:
#   SMOKE_PORT  backend port (default 8082)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$CLI_DIR/.." && pwd)"
BACKEND_DIR="$REPO_ROOT/backend"

PORT="${SMOKE_PORT:-8082}"
URL="http://127.0.0.1:${PORT}"
USERNAME="ci@traceway.local"
PASSWORD="ci-smoke-password"
PROJECT_TOKEN="ci-smoke-token"

DRY_RUN=0
for arg in "$@"; do
  case "$arg" in
    --dry-run) DRY_RUN=1 ;;
    *) echo "unknown argument: $arg" >&2; exit 2 ;;
  esac
done

log() { printf 'smoke-ci: %s\n' "$*"; }
die() { printf 'smoke-ci: error: %s\n' "$*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die "go not found on PATH"
command -v jq >/dev/null 2>&1 || die "jq not found on PATH (install jq)"
command -v curl >/dev/null 2>&1 || die "curl not found on PATH"
[ -f "$BACKEND_DIR/go.mod" ] || die "backend module not found at $BACKEND_DIR"
[ -d "$BACKEND_DIR/cmd/testserver" ] || die "backend testserver not found (cmd/testserver)"
[ -d "$CLI_DIR/test/smoke" ] || die "smoke tests not found at $CLI_DIR/test/smoke"

if [ "$DRY_RUN" -eq 1 ]; then
  log "dry-run OK: go/jq/curl present, backend at $BACKEND_DIR, smoke tests present"
  log "dry-run OK: would seed user $USERNAME + project, serve on $URL, run go test -tags smoke"
  exit 0
fi

WORKDIR="$(mktemp -d)"
SERVER_LOG="$WORKDIR/server.log"
SERVER_PID=""

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

log "building embedded backend"
( cd "$BACKEND_DIR" && go build -o "$WORKDIR/testserver" ./cmd/testserver )

log "starting backend (sqlite) on $URL"
(
  cd "$WORKDIR"
  TESTSERVER_PORT="$PORT" \
  TESTSERVER_SQLITE_PATH="$WORKDIR/traceway.db" \
  TESTSERVER_USER="$USERNAME" \
  TESTSERVER_PASSWORD="$PASSWORD" \
  TESTSERVER_PROJECT_TOKEN="$PROJECT_TOKEN" \
  exec ./testserver
) >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!

log "waiting for backend health"
# Up to 90s: a cold CI runner may still be compiling the backend on first run.
healthy=0
for _ in $(seq 1 90); do
  if curl -fsS "$URL/health" >/dev/null 2>&1; then
    healthy=1
    break
  fi
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then
    echo "--- backend log ---" >&2
    cat "$SERVER_LOG" >&2
    die "backend exited before becoming healthy"
  fi
  sleep 1
done
[ "$healthy" -eq 1 ] || { cat "$SERVER_LOG" >&2; die "backend never became healthy on $URL"; }

log "discovering seeded project id via the CLI"
( cd "$CLI_DIR" && go build -o "$WORKDIR/traceway" ./cmd/traceway )

export XDG_CONFIG_HOME="$WORKDIR/xdg-config"
export XDG_STATE_HOME="$WORKDIR/xdg-state"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_STATE_HOME"

printf '%s\n' "$PASSWORD" | "$WORKDIR/traceway" login --url "$URL" --username "$USERNAME" --password-stdin
if ! PROJECTS_JSON="$("$WORKDIR/traceway" projects list --output json)"; then
  die "'projects list' failed: $PROJECTS_JSON"
fi
PROJECT_ID="$(printf '%s' "$PROJECTS_JSON" | jq -r '.[0].id // empty')"
[ -n "$PROJECT_ID" ] || die "could not resolve the seeded project id; 'projects list' returned: $PROJECTS_JSON"
log "seeded project id: $PROJECT_ID"

log "running smoke suite against $URL"
cd "$CLI_DIR"
TRACEWAY_SMOKE_URL="$URL" \
TRACEWAY_SMOKE_USERNAME="$USERNAME" \
TRACEWAY_SMOKE_PASSWORD="$PASSWORD" \
TRACEWAY_SMOKE_PROJECT_ID="$PROJECT_ID" \
go test -tags smoke -count=1 ./test/smoke/...

log "smoke suite passed"
