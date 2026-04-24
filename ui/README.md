# LocalMaps UI

Next.js 15 (App Router) frontend for the LocalMaps self-hosted maps
platform. Renders a MapLibre canvas with Google-Maps-style chrome over
data served by the Go API gateway in `../server`.

## Local development

```bash
npm install
NEXT_PUBLIC_GATEWAY_URL=http://localhost:8080 npm run dev
```

The dev server proxies `/api/*`, `/og/*` and `/embed` to the gateway URL
so the UI can run on a different port without CORS friction.

## Scripts

| Command            | Purpose                                          |
| ------------------ | ------------------------------------------------ |
| `npm run dev`      | Next.js dev server (port 3000).                  |
| `npm run build`    | Production build.                                |
| `npm run start`    | Start the production server.                     |
| `npm run test`     | Vitest unit tests (jsdom).                       |
| `npm run lint`     | ESLint via `next lint`.                          |
| `npm run typecheck`| `tsc --noEmit`.                                  |
| `npm run typegen`  | Regenerate `../contracts/ts/api.d.ts` from OpenAPI. |
| `npm run format`   | Prettier.                                        |

## Architecture

See `../docs/01-architecture.md`. Key UI pieces:

- `app/page.tsx`               — main map page (Google Maps-style chrome).
- `app/embed/page.tsx`         — minimal embed viewer for iframes.
- `app/admin/**`               — admin shell for Regions, Settings, Jobs.
- `components/map/MapView.tsx` — the single MapLibre instance.
- `components/chrome/*`        — search, panels, FABs, attribution,
                                  right-click menu.
- `lib/api/*`                  — zod-validated fetch wrapper + hooks.
- `lib/state/map.ts`           — Zustand store for client-only map state.
- `lib/url-state.ts`           — `#zoom/lat/lon/bearing/pitch` round-trip.

## Contracts

All HTTP calls go through `lib/api/client.ts`, which validates every
response against a hand-written zod schema in `lib/api/schemas.ts`. The
zod schemas mirror `../contracts/openapi.yaml` — if a field isn't there,
it isn't in the UI either (see `../docs/06-agent-rules.md` R2).
