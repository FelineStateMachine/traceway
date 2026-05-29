#!/usr/bin/env bash
# Assembles the CLI install site: pins the release tag into install.sh and
# install.ps1, then stages them (plus uninstall.sh) into an output directory.
#
# Used by .github/workflows/publish-install-cli.yml; runnable locally:
#
#   cli/scripts/assemble-install-site.sh cli/v1.2.3 ./site
#
# The substitution is anchored to the literal assignment line so it never
# rewrites the placeholder occurrences in the not-released safety checks.

set -euo pipefail

TAG="${1:?usage: assemble-install-site.sh <tag> <outdir>}"
OUT="${2:?usage: assemble-install-site.sh <tag> <outdir>}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$OUT"

sed "s|DEFAULT_TAG=\"__TRACEWAY_CLI_TAG__\"|DEFAULT_TAG=\"${TAG}\"|" \
    "$SCRIPT_DIR/install.sh" > "$OUT/install.sh"
sed "s|else { '__TRACEWAY_CLI_TAG__' }|else { '${TAG}' }|" \
    "$SCRIPT_DIR/install.ps1" > "$OUT/install.ps1"
cp "$SCRIPT_DIR/uninstall.sh" "$OUT/uninstall.sh"
chmod 0644 "$OUT/install.sh" "$OUT/install.ps1" "$OUT/uninstall.sh"

grep -q "^DEFAULT_TAG=\"${TAG}\"$" "$OUT/install.sh" \
  || { echo "install.sh substitution did not match"; exit 1; }
grep -q "else { '${TAG}' }" "$OUT/install.ps1" \
  || { echo "install.ps1 substitution did not match"; exit 1; }

echo "assembled install site for ${TAG} into ${OUT}"
ls -la "$OUT"
