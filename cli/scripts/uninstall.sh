#!/usr/bin/env bash
# Traceway CLI uninstaller (Linux / macOS).
#
#   curl -fsSL https://cli.tracewayapp.com/uninstall.sh | bash
#
# Optional:
#   TRACEWAY_BIN_DIR   Directory the binary was installed to (default /usr/local/bin).

set -eu

err() { printf 'traceway-uninstall: error: %s\n' "$*" >&2; exit 1; }
log() { printf 'traceway-uninstall: %s\n' "$*"; }

BIN_DIR="${TRACEWAY_BIN_DIR:-/usr/local/bin}"
BIN_PATH="${BIN_DIR}/traceway"

if [ ! -e "$BIN_PATH" ]; then
  resolved="$(command -v traceway 2>/dev/null || true)"
  [ -n "$resolved" ] && BIN_PATH="$resolved"
fi

[ -e "$BIN_PATH" ] || { log "traceway not found (nothing to remove)"; exit 0; }

SUDO=""
if [ ! -w "$(dirname "$BIN_PATH")" ] && [ "$(id -u)" -ne 0 ]; then
  command -v sudo >/dev/null 2>&1 || err "$(dirname "$BIN_PATH") is not writable and sudo is unavailable"
  SUDO="sudo"
fi

log "removing $BIN_PATH"
$SUDO rm -f "$BIN_PATH"

log "traceway removed"
log "note: per-profile config/state under \$XDG_CONFIG_HOME and \$XDG_STATE_HOME was left in place"
