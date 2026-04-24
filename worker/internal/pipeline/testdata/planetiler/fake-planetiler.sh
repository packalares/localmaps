#!/bin/sh
# fake-planetiler.sh — a tiny stand-in for `java -jar planetiler.jar ...`.
# Emits progress lines on stderr the parser recognises, then exits 0.
#
# Invoked as:  fake-planetiler.sh <mode> [args...]
#   mode=ok       — emit a few progress lines + exit 0
#   mode=fail     — emit lines + exit 2 (tests PlanetilerError)
#   mode=hang     — sleep forever (tests ctx cancellation)
#
# Any surplus args are ignored (this script doesn't care about real
# planetiler CLI flags).

mode="${1:-ok}"

case "$mode" in
ok)
    printf 'stdout hello\n'
    printf '12:00 INFO  [osm_pass1] -  10%% [0s] starting\n' 1>&2
    printf '12:00 INFO  [osm_pass1] -  50%% [1s] nodes: 3M\n' 1>&2
    printf '12:00 INFO  [osm_pass1] - 100%% [2s] done\n' 1>&2
    printf '12:00 INFO  [osm_pass2] -  50%% [3s] ways: 1M\n' 1>&2
    printf '12:00 INFO  [mbtiles]   - 100%% [4s] writing\n' 1>&2
    exit 0
    ;;
fail)
    printf '12:00 INFO  [osm_pass1] -   5%% [0s] starting\n' 1>&2
    printf '12:00 ERROR boom: something went wrong\n' 1>&2
    exit 2
    ;;
hang)
    # Sleep for a very long time; the test cancels its ctx.
    # Trap SIGTERM so we exit cleanly after a signal.
    trap 'exit 0' TERM
    sleep 60 &
    wait
    ;;
*)
    echo "unknown mode: $mode" 1>&2
    exit 3
    ;;
esac
