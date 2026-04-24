#!/bin/sh
# entrypoint.sh — PID 1 supervisor for the gateway + worker containers.
#
# Usage: entrypoint.sh <binary-name>  (e.g. `entrypoint.sh gateway`)
#
# Prefers /live/<bin> (hostPath dev mount) when present; otherwise runs
# the image-baked /app/<bin>. Loops: when the supervised binary exits,
# re-exec picks up a new binary written to /live since the last run.
#
# Dev flow:
#   go build -o /tmp/gateway ./server/cmd/localmaps
#   scp /tmp/gateway root@gpu:/packalares/dev/localmaps/gateway
#   kubectl exec ... -c gateway -- pkill -f /live/gateway || true
#   # supervisor re-execs → new binary runs, no pod restart
#
# Production (no /live mount): just the baked binary runs in a loop
# that restarts on crash — same behaviour Kubernetes expects.
set -eu

NAME="${1:-gateway}"
BAKED="/app/${NAME}"
LIVE="/live/${NAME}"

echo "[entrypoint] starting supervisor for ${NAME}"

# shellcheck disable=SC2317
while :; do
    if [ -x "${LIVE}" ]; then
        BIN="${LIVE}"
    elif [ -x "${BAKED}" ]; then
        BIN="${BAKED}"
    else
        echo "[entrypoint] no executable at ${LIVE} or ${BAKED}; sleeping" >&2
        sleep 10
        continue
    fi
    echo "[entrypoint] exec ${BIN}"
    "${BIN}" || RC=$?
    echo "[entrypoint] ${NAME} exited (rc=${RC:-0}); restarting in 1s" >&2
    sleep 1
done
