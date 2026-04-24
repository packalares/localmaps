#!/usr/bin/env sh
# Validate the dev compose file resolves cleanly against .env.example.
# Exits non-zero on any compose-level syntax or reference error.
set -eu
HERE="$(cd "$(dirname "$0")" && pwd)"
docker compose -f "$HERE/docker-compose.yml" --env-file "$HERE/.env.example" config > /dev/null
echo OK
