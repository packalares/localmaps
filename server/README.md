# LocalMaps gateway (server)

Go binary that serves the LocalMaps HTTP + WebSocket API. This is the
only service browsers talk to — upstream engines (Pelias, Valhalla,
Protomaps, Redis) live on the internal network.

See `../docs/` for the full picture:

- `00-project-charter.md` — what we're building.
- `01-architecture.md` — where this package fits.
- `03-contracts.md` + `../contracts/openapi.yaml` — authoritative API.
- `04-data-model.md` — SQLite schema.
- `07-config-schema.md` — boot env + runtime settings.

## Build + run locally

```bash
cd apps/localmaps/server
make build
LOCALMAPS_DATA_DIR=$PWD/.data ./bin/localmaps
```

Defaults will create `.data/config.db` on first run, seed every setting
listed in `docs/07-config-schema.md`, and listen on `:8080`. Visit:

- `GET /api/health` — liveness
- `GET /api/ready` — dependency readiness (dials Redis/Pelias/Valhalla/Protomaps)
- `GET /api/version` — build info
- `GET /metrics` — Prometheus
- `GET /api/settings/schema` — anonymous settings schema

All feature endpoints (tiles/search/route/pois/regions/settings/share)
are wired but return `501 Not Implemented` in Phase 1 — implementations
land in Phase 2+ per `docs/05-phases.md`.

## Worker mode

The same repo also builds a separate worker binary at
`../worker/cmd/worker/`. `LOCALMAPS_MODE=worker` on the gateway
exits early so the cluster can schedule the worker binary in its own
pod (see `01-architecture.md`).

## Tests

```bash
make test
```

Unit tests cover the config store migrations, safepath traversal
defence, auth middleware, rate-limit semantics, and the error envelope
shape. Anything past a thin proxy must have tests per R11.
