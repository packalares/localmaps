# deploy/

Everything needed to run LocalMaps on a laptop or server with Docker
Compose. A standalone Helm chart (for Kubernetes deployments) lives
in a sibling repository and is outside this folder's scope.

## What lives here

| Path | Purpose |
|---|---|
| `Dockerfile.gateway` | Multi-stage build of the Go API gateway binary. |
| `Dockerfile.worker`  | Multi-stage build of the Go region worker binary. |
| `Dockerfile.ui`      | **Dev convenience only.** See "UI image" below. |
| `docker-compose.yml` | Full local stack (gateway + worker + deps). |
| `.env.example`       | Every boot-time env var the Go binaries accept. |
| `compose-validate.sh`| One-shot `docker compose config` smoke test. |
| `pelias/pelias.json` | Pelias API config (minimal, adapted from upstream). |

## Quick start

```sh
# from the repo-app root:
cp deploy/.env.example deploy/.env         # optional — the file already has working defaults
docker compose -f deploy/docker-compose.yml up --build
open http://localhost:8080
```

Only port `8080` is bound to the host — the gateway is the single
public surface. `protomaps`, `valhalla`, `pelias-*`, and `redis` are
reachable only on the compose network by service name.

## Stop / reset

```sh
docker compose -f deploy/docker-compose.yml down        # stop
docker compose -f deploy/docker-compose.yml down -v     # stop + drop volumes (there are none declared)
rm -rf deploy/data                                      # wipe all persistent state
```

`./data/` under this directory is the **single** persistence path
(mirrors `/data` inside every container, matching
`docs/04-data-model.md`). Delete it to return to a fresh install.

## Resource budget warning

This stack is heavier than it looks:

- **Pelias Elasticsearch** requests ~2 GiB of JVM heap
  (`ES_JAVA_OPTS=-Xms1g -Xmx1g`) plus OS cache; budget ≥ 3 GiB free RAM
  before starting.
- **Valhalla** tile build is CPU-bound and can spike all cores.
- First region import is bandwidth-heavy — Geofabrik extracts run
  50 MB – 5 GB depending on the region.

On a 16 GiB dev box you can run the stack comfortably with one small
country installed. For anything continent-sized, use a Kubernetes
deployment (see the sibling Helm chart repository).

## UI image

`Dockerfile.ui` is **optional**. The production path (per
`docs/10-deploy.md`) is: the Go gateway serves the built Next.js app
as embedded static content, so the chart ships ONE image (the
gateway). The `ui` Dockerfile exists purely to let a dev run a
standalone Next.js container during UI-layer work; `docker-compose.yml`
does NOT start it. Enable it manually with:

```sh
docker build -f deploy/Dockerfile.ui -t localmaps/ui:dev ..
docker run --rm -p 3000:3000 localmaps/ui:dev
```

## Gateway + worker share one image

The `worker` service in `docker-compose.yml` reuses `localmaps/gateway:dev`
with `LOCALMAPS_MODE=worker` — this matches `docs/10-deploy.md`
("same Go binary in worker mode") and avoids a duplicate build.
`Dockerfile.worker` is kept for the day the worker grows its own
`cmd/worker/main.go`; swap the service over then.

## Validation

```sh
./deploy/compose-validate.sh
```

That runs `docker compose config` against `.env.example` and prints
`OK` on success. Use it before committing compose changes.

## Helm chart — out of scope for this folder

The Helm chart is maintained in a separate repository. It runs the
**same images** built here, with the **same env var surface** from
`.env.example`, and mounts a hostPath (or equivalent PVC) at
`/data`.

## TODO / known gaps

- `protomaps/go-pmtiles`, `ghcr.io/gis-ops/docker-valhalla/valhalla`,
  `pelias/api` image tags are plausible current releases but need a
  `@sha256:` digest pin (docs/08-security.md) before prod.
- `pelias/pelias.json` wires the minimum to let pelias-api boot
  against an empty ES; `imports.*` paths are placeholders for the
  worker's Pelias import step (Phase 4 wiring).
- `ui/` does not yet commit a `pnpm-lock.yaml`; once it does, flip
  `Dockerfile.ui` to `pnpm install --frozen-lockfile`.
- `./worker/cmd/worker` is empty; the worker container runs the
  gateway binary in `LOCALMAPS_MODE=worker` until that lands.
