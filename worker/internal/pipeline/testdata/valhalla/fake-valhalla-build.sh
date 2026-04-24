#!/usr/bin/env bash
# Fake valhalla_build_* stub for unit tests.
#
# Behaviour:
#   - Prints a predictable progress pattern on stderr (0%, 25%, 50%,
#     75%, 100%) with small sleeps in between so the scanner has time
#     to flush every line.
#   - Exits 0 by default; exits 1 when FAKE_VALHALLA_FAIL=1 is set in
#     the environment.
#   - Ignores its actual arguments — tests only care that the runner
#     launched it and captured the progress lines.
#
# Not executed by the production binary; only by go test via
# WithValhallaExecutables.

set -euo pipefail

for pct in 0 25 50 75 100; do
  echo "[INFO] Processing fake stage ${pct}%" 1>&2
  # Tiny sleep keeps the bufio scanner from coalescing our lines into a
  # single read — the scanner tests want to see each line emerge.
  sleep 0.02
done

if [[ "${FAKE_VALHALLA_FAIL:-0}" == "1" ]]; then
  echo "[ERROR] fake: FAKE_VALHALLA_FAIL=1, exiting non-zero" 1>&2
  exit 1
fi

exit 0
