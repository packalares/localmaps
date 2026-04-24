# LocalMaps

Self-hostable maps platform — open data, open stack. Covers the core Google
Maps feature set without cloud dependencies. A single-node Docker Compose
stack is the default deployment; the app is fully standalone (native
session-cookie auth, no external reverse-proxy required).

## Status

In active construction. See `docs/05-phases.md` for phase status.

## What it does

- Pan/zoom/rotate vector map with OSM data (via Protomaps)
- Search places (streets, POIs, cities) — Pelias-backed
- Turn-by-turn directions (car, bike, foot, truck) — Valhalla-backed
- Points-of-interest with Overture + OSM tags
- Shareable URLs (`/m/<lat>,<lon>,<zoom>/...`) + embeddable iframe
- Admin UI to browse + download per-country / per-continent OSM extracts
- Per-region update scheduler (daily / weekly / monthly / custom)
- Every knob is configurable — no hardcoding

## Non-goals

- Satellite / aerial imagery (no free planet-scale source)
- Street View (no open crowd-sourced dataset at Google coverage)
- Real-time traffic (no data source without a user base)
- Exact visual parity with Google Maps — different map style, close UX

## Stack

- **Frontend**: Next.js (React) + MapLibre GL JS + Tailwind
- **Backend**: Go (Fiber router)
- **Tile server**: go-pmtiles
- **Routing**: Valhalla
- **Geocoding**: Pelias
- **POI**: Overture Maps (Parquet + DuckDB)
- **Worker queue**: Asynq (Redis-backed)
- **Config DB**: SQLite
- **Deploy**: Docker Compose (self-hosted) or a standalone Helm chart

Full stack rationale in `docs/02-stack.md`.

## Quick tour of `docs/`

| File | Purpose |
|---|---|
| `00-project-charter.md` | Mission, scope, non-goals, success criteria |
| `01-architecture.md` | System design + data flow |
| `02-stack.md` | Locked tech decisions |
| `03-contracts.md` | Every API contract — prevents endpoint invention |
| `04-data-model.md` | DB schema + region state machine |
| `05-phases.md` | Phase plan + dependencies |
| `06-agent-rules.md` | Rules every contributing agent must follow |
| `07-config-schema.md` | Every configurable setting |
| `08-security.md` | Rate limits, auth, secrets |
| `09-testing.md` | Test strategy |
| `10-deploy.md` | Docker Compose + Helm deployment specs |
