#!/usr/bin/env bash
# Capture the SUT's own crash evidence before the box is destroyed. Run after
# the loadgen finishes (and from the teardown trap), so a mid-run OOM leaves a
# definitive record instead of us inferring it from app-side memory samples —
# which read low because they're taken at step boundaries, after the backlog
# drained or after the process already died.
#
# Collects, over SSH:
#   - kernel OOM kills from dmesg (the "Killed process ... anon-rss:NkB" line
#     gives the exact resident memory at time of death — the ground truth)
#   - the traceway container's exit code (137 = SIGKILL/OOM), OOMKilled flag,
#     and restart count
#   - the last ~200 backend log lines (catches a Go "fatal error: out of
#     memory" or a panic stack if it died in-process rather than via the kernel)
#
# Usage: sut-forensics.sh <sut-public-ip> <mode> <out-file>
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=_ssh.sh
source "${SCRIPT_DIR}/_ssh.sh"

if [[ $# -lt 3 ]]; then
    echo "usage: $0 <sut-public-ip> <mode> <out-file>" >&2
    exit 2
fi
SUT_IP="$1"
MODE="$2"
OUT="$3"
COMPOSE="benchmarks/compose/docker-compose.${MODE}.yml"

remote_script=$(cat <<'REMOTE'
echo "===== host: uname / memory ====="
uname -a
free -m

echo
echo "===== kernel OOM kills (dmesg) ====="
oom=$( { dmesg -T 2>/dev/null || dmesg 2>/dev/null; } | grep -iE "killed process|out of memory|oom-kill|oom_reaper|invoked oom-killer" | tail -40 )
if [ -n "$oom" ]; then echo "$oom"; else echo "(no OOM kills in dmesg — backend did not die to the kernel OOM-killer)"; fi

echo
echo "===== traceway container state ====="
cd /opt/traceway 2>/dev/null || { echo "(/opt/traceway missing)"; exit 0; }
docker compose -f __COMPOSE__ ps -a
cid=$(docker compose -f __COMPOSE__ ps -aq traceway 2>/dev/null | head -1)
if [ -n "$cid" ]; then
  docker inspect --format 'Status={{.State.Status}} ExitCode={{.State.ExitCode}} OOMKilled={{.State.OOMKilled}} RestartCount={{.RestartCount}} Error={{.State.Error}}' "$cid"
else
  echo "(no traceway container found)"
fi

echo
echo "===== last 200 backend log lines ====="
docker compose -f __COMPOSE__ logs --tail=200 --no-color traceway 2>&1 | tail -200
REMOTE
)
remote_script="${remote_script//__COMPOSE__/${COMPOSE}}"

{
    echo "# SUT forensics: ip=${SUT_IP} mode=${MODE} captured=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
    echo
    bench_ssh "${SUT_IP}" "bash -s" <<<"${remote_script}" 2>&1 \
        || echo "(forensics ssh failed — SUT ${SUT_IP} unreachable)"
} >"${OUT}"

echo "wrote SUT forensics -> ${OUT}" >&2
