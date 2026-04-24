/**
 * Public entry for `@localmaps/contracts`.
 *
 * Re-exports:
 *   - Raw generated OpenAPI types (`paths`, `components`, `operations`).
 *   - Narrow aliases for every schema defined in `openapi.yaml`, so
 *     consumers can write `import type { Region, Job } from '@localmaps/contracts'`.
 *   - Zod validators (via `./schemas`) for runtime validation of
 *     response bodies.
 *   - WebSocket event types (via `./events`).
 *   - `ErrorCode` const-object + type (via `./errors`).
 *
 * Source of truth: `contracts/openapi.yaml`. Do NOT hand-edit `api.d.ts`
 * — regenerate with `pnpm run typegen`.
 */

import type { components, paths, operations } from './api.js';

export type { components, paths, operations };

// --------------------------------------------------------------------
// Narrow schema aliases — one per schema defined in openapi.yaml.
// Keep this list in lockstep with `components.schemas` in the YAML.
// `scripts/check-parity.ts` enforces the bijection at build time.
//
// NOTE: `ErrorCode` is intentionally NOT re-exported here as a type
// alias — the const-object-plus-type pattern in `./errors.ts` owns
// the public `ErrorCode` name so consumers get both value and type
// from a single import.
// --------------------------------------------------------------------

export type TraceID = components['schemas']['TraceID'];
export type ErrorResponse = components['schemas']['ErrorResponse'];
export type LatLon = components['schemas']['LatLon'];
export type BBox = components['schemas']['BBox'];
export type RegionState = components['schemas']['RegionState'];
export type RegionSchedule = components['schemas']['RegionSchedule'];
export type Region = components['schemas']['Region'];
export type RegionCatalogEntry = components['schemas']['RegionCatalogEntry'];
export type JobState = components['schemas']['JobState'];
export type JobKind = components['schemas']['JobKind'];
export type Job = components['schemas']['Job'];
export type GeocodeResult = components['schemas']['GeocodeResult'];
export type RouteMode = components['schemas']['RouteMode'];
export type RouteRequest = components['schemas']['RouteRequest'];
export type RouteLeg = components['schemas']['RouteLeg'];
export type Route = components['schemas']['Route'];
export type RouteResponse = components['schemas']['RouteResponse'];
export type IsochroneRequest = components['schemas']['IsochroneRequest'];
export type MatrixRequest = components['schemas']['MatrixRequest'];
export type MatrixResponse = components['schemas']['MatrixResponse'];
export type Poi = components['schemas']['Poi'];
export type PoiCategory = components['schemas']['PoiCategory'];
export type SettingsTree = components['schemas']['SettingsTree'];
export type SettingsSchemaNode = components['schemas']['SettingsSchemaNode'];
export type SettingsSchemaResponse = components['schemas']['SettingsSchemaResponse'];
export type ShortLink = components['schemas']['ShortLink'];
export type AuthUser = components['schemas']['AuthUser'];
export type AuthMeResponse = components['schemas']['AuthMeResponse'];

// --------------------------------------------------------------------
// Re-export sub-modules.
// `./errors` exports both a const-object `ErrorCode` and a type
// `ErrorCode` inferred from it. `./schemas` exports zod validators.
// `./events` exports the WS event discriminated-union.
// --------------------------------------------------------------------

export * from './errors.js';
export * from './schemas.js';
export * from './events.js';
