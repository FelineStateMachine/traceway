#!/usr/bin/env bash
# Traceway CLI installer (Linux / macOS).
#
#   curl -fsSL https://cli.tracewayapp.com/install.sh | bash
#
# Optional:
#   TRACEWAY_CLI_VERSION   Install a specific version (X.Y.Z) instead of the
#                          version this installer was published with.
#   TRACEWAY_BIN_DIR       Install directory (default /usr/local/bin).
#   TRACEWAY_RELEASES_URL  Override the release-download base URL (default:
#                          GitHub Releases). Accepts file:// URLs for
#                          air-gapped / test installs.

set -eu

# __TRACEWAY_CLI_TAG__ is replaced by scripts/assemble-install-site.sh (driven
# by .github/workflows/publish-install-cli.yml) when the script is deployed.
DEFAULT_TAG="__TRACEWAY_CLI_TAG__"

REPO="tracewayapp/traceway"
RELEASES="${TRACEWAY_RELEASES_URL:-https://github.com/${REPO}/releases/download}"

err() { printf 'traceway-install: error: %s\n' "$*" >&2; exit 1; }
log() { printf 'traceway-install: %s\n' "$*"; }

if [ -n "${TRACEWAY_CLI_VERSION:-}" ]; then
  TAG="cli/v${TRACEWAY_CLI_VERSION}"
else
  TAG="$DEFAULT_TAG"
fi

case "$TAG" in
  ""|__TRACEWAY_CLI_TAG__|__NOT_RELEASED__)
    err "this installer has not been released yet. Check https://github.com/${REPO}/releases, then re-run with TRACEWAY_CLI_VERSION=X.Y.Z."
    ;;
esac

VERSION="${TAG#cli/v}"

UNAME_S="$(uname -s)"
UNAME_M="$(uname -m)"
case "$UNAME_S" in
  Linux)  OS="linux"  ;;
  Darwin) OS="darwin" ;;
  *) err "unsupported OS: $UNAME_S (Linux and macOS only; for Windows use install.ps1)" ;;
esac
case "$UNAME_M" in
  x86_64|amd64)  LABEL="x86_64" ;;
  arm64|aarch64) LABEL="arm64"  ;;
  *) err "unsupported arch: $UNAME_M" ;;
esac
log "detected $OS/$LABEL"

BIN_DIR="${TRACEWAY_BIN_DIR:-/usr/local/bin}"
BIN_PATH="${BIN_DIR}/traceway"

# Use sudo only when the target isn't writable as the current user.
SUDO=""
need_sudo=1
if [ -d "$BIN_DIR" ] && [ -w "$BIN_DIR" ]; then
  need_sudo=0
elif [ ! -d "$BIN_DIR" ] && [ -w "$(dirname "$BIN_DIR")" ]; then
  need_sudo=0
fi
if [ "$need_sudo" -eq 1 ] && [ "$(id -u)" -ne 0 ]; then
  command -v sudo >/dev/null 2>&1 \
    || err "$BIN_DIR is not writable and sudo is unavailable; set TRACEWAY_BIN_DIR to a writable directory"
  SUDO="sudo"
fi

TARBALL="traceway_${VERSION}_${OS}_${LABEL}.tar.gz"
URL="${RELEASES}/${TAG}/${TARBALL}"
CHECKSUMS_URL="${RELEASES}/${TAG}/checksums.txt"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

log "downloading $URL"
curl -fsSL -o "${TMP}/${TARBALL}" "$URL" || err "failed to download $URL"

log "verifying sha256"
curl -fsSL -o "${TMP}/checksums.txt" "$CHECKSUMS_URL" || err "failed to download checksums.txt"
EXPECTED="$(awk -v f="$TARBALL" '$2==f || $2=="*"f {print $1}' "${TMP}/checksums.txt")"
[ -n "$EXPECTED" ] || err "no checksum entry for $TARBALL in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMP}/${TARBALL}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMP}/${TARBALL}" | awk '{print $1}')"
else
  err "no sha256sum or shasum available on this host"
fi
[ "$EXPECTED" = "$ACTUAL" ] || err "checksum mismatch for $TARBALL (expected $EXPECTED, got $ACTUAL)"

log "unpacking"
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"
[ -f "${TMP}/traceway" ] || err "binary not found in archive (expected ./traceway)"

log "installing → $BIN_PATH"
$SUDO mkdir -p "$BIN_DIR"
$SUDO install -m 0755 "${TMP}/traceway" "$BIN_PATH"

log "installed traceway ${VERSION} → ${BIN_PATH}"
if ! command -v traceway >/dev/null 2>&1; then
  log "note: ${BIN_DIR} is not on your PATH — add it, then re-open your shell"
fi
log "next: traceway login --url <your-traceway-url>"
