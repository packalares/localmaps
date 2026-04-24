#!/usr/bin/env bash
# fake-importer.sh — tiny stand-in for the Pelias openstreetmap importer
# used by pelias_test.go. Emits the exact progress format the real
# importer does, so the stderr scanner is exercised end-to-end:
#
#   [INFO] openstreetmap: imported N/M
#
# If PELIAS_FAIL=1 is set, exits non-zero after a couple of progress
# lines. Otherwise walks 0..M in a few steps and exits 0.
set -eu

# The Go test sets PELIAS_CONFIG to the generated pelias.json. Verify
# it's non-empty — catches regressions in writeFileAtomic.
if [[ -n "${PELIAS_CONFIG:-}" ]]; then
  if [[ ! -s "${PELIAS_CONFIG}" ]]; then
    echo "fake-importer: PELIAS_CONFIG=${PELIAS_CONFIG} is empty" >&2
    exit 2
  fi
fi

total=${PELIAS_FAKE_TOTAL:-1000}
steps=${PELIAS_FAKE_STEPS:-5}
step=$(( total / steps ))

echo "fake-importer: starting" >&2
for ((i=1; i<=steps; i++)); do
  echo "fake-importer: writing data row" >&1
  n=$(( step * i ))
  echo "[INFO] openstreetmap: imported ${n}/${total}" >&2
  if [[ "${PELIAS_FAIL:-0}" == "1" && "${i}" -ge 2 ]]; then
    echo "fake-importer: simulated failure" >&2
    exit 1
  fi
done
echo "[INFO] openstreetmap: imported ${total}/${total}" >&2
echo "fake-importer: done" >&2
exit 0
