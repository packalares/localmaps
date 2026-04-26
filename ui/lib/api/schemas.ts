import { z } from "zod";

/**
 * Hand-written zod schemas that mirror the shapes defined in
 * `contracts/openapi.yaml`. These validate every HTTP response at runtime.
 *
 * Rules:
 * - Never add a field that is not in `openapi.yaml`.
 * - When the spec says a field is nullable, use `.nullable()`.
 * - When the spec marks a field optional, use `.optional()`.
 *
 * See `docs/06-agent-rules.md` R2.
 */

export const TraceIdSchema = z.string();

export const ErrorCodeSchema = z.enum([
  "BAD_REQUEST",
  "UNAUTHORIZED",
  "FORBIDDEN",
  "NOT_FOUND",
  "CONFLICT",
  "RATE_LIMITED",
  "INTERNAL",
  "UPSTREAM_UNAVAILABLE",
  "REGION_NOT_READY",
  "REGION_NOT_INSTALLED",
  "REGION_OUT_OF_COVERAGE",
  "INVALID_REGION_NAME",
  "JOB_NOT_FOUND",
  "SCHEMA_MISMATCH",
  "EGRESS_DENIED",
]);

export const ErrorResponseSchema = z.object({
  error: z.object({
    code: ErrorCodeSchema,
    message: z.string(),
    retryable: z.boolean(),
    details: z.record(z.unknown()).optional(),
  }),
  traceId: TraceIdSchema,
});
export type ErrorResponse = z.infer<typeof ErrorResponseSchema>;

export const LatLonSchema = z.object({
  lat: z.number().min(-90).max(90),
  lon: z.number().min(-180).max(180),
});
export type LatLon = z.infer<typeof LatLonSchema>;

export const BBoxSchema = z.tuple([
  z.number(),
  z.number(),
  z.number(),
  z.number(),
]);
export type BBox = z.infer<typeof BBoxSchema>;

export const GeocodeResultSchema = z.object({
  id: z.string(),
  label: z.string(),
  category: z.string().nullable().optional(),
  address: z.record(z.string()).optional(),
  center: LatLonSchema,
  bbox: BBoxSchema.nullable().optional(),
  confidence: z.number().min(0).max(1),
  region: z.string().nullable().optional(),
});
export type GeocodeResult = z.infer<typeof GeocodeResultSchema>;

export const GeocodeAutocompleteResponseSchema = z.object({
  results: z.array(GeocodeResultSchema),
  traceId: TraceIdSchema,
});
export type GeocodeAutocompleteResponse = z.infer<
  typeof GeocodeAutocompleteResponseSchema
>;

export const GeocodeSearchResponseSchema = GeocodeAutocompleteResponseSchema;
export type GeocodeSearchResponse = z.infer<typeof GeocodeSearchResponseSchema>;

export const GeocodeReverseResponseSchema = z.object({
  result: GeocodeResultSchema,
  traceId: TraceIdSchema,
});
export type GeocodeReverseResponse = z.infer<typeof GeocodeReverseResponseSchema>;

export const RouteModeSchema = z.enum([
  "auto",
  "bicycle",
  "pedestrian",
  "truck",
]);
export type RouteMode = z.infer<typeof RouteModeSchema>;

export const RouteRequestSchema = z.object({
  locations: z.array(LatLonSchema).min(2),
  mode: RouteModeSchema,
  avoidHighways: z.boolean().optional(),
  avoidTolls: z.boolean().optional(),
  avoidFerries: z.boolean().optional(),
  alternatives: z.number().int().min(0).max(5).optional(),
  units: z.enum(["metric", "imperial"]).optional(),
  language: z.string().nullable().optional(),
  truck: z
    .object({
      heightMeters: z.number().optional(),
      widthMeters: z.number().optional(),
      weightTons: z.number().optional(),
      lengthMeters: z.number().optional(),
    })
    .nullable()
    .optional(),
});
export type RouteRequest = z.infer<typeof RouteRequestSchema>;

export const RouteLegSchema = z.object({
  summary: z
    .object({
      timeSeconds: z.number().optional(),
      distanceMeters: z.number().optional(),
    })
    .optional(),
  maneuvers: z.array(
    z.object({
      instruction: z.string(),
      beginShapeIndex: z.number().int(),
      endShapeIndex: z.number().int().optional(),
      distanceMeters: z.number().optional(),
      timeSeconds: z.number().optional(),
      type: z.string().optional(),
      streetName: z.string().nullable().optional(),
    }),
  ),
  geometry: z.object({
    polyline: z.string(),
  }),
});
export type RouteLeg = z.infer<typeof RouteLegSchema>;

export const RouteSchema = z.object({
  id: z.string(),
  summary: z
    .object({
      timeSeconds: z.number().optional(),
      distanceMeters: z.number().optional(),
    })
    .optional(),
  legs: z.array(RouteLegSchema),
  waypoints: z.array(LatLonSchema).optional(),
  mode: RouteModeSchema.optional(),
});
export type Route = z.infer<typeof RouteSchema>;

export const RouteResponseSchema = z.object({
  routes: z.array(RouteSchema),
  traceId: TraceIdSchema,
});
export type RouteResponse = z.infer<typeof RouteResponseSchema>;

// POST /api/isochrone — body shape per IsochroneRequest in openapi.yaml.
// The response is an unconstrained GeoJSON FeatureCollection (openapi
// declares the response as `application/json: {}`), so we validate only
// the minimal GeoJSON envelope needed to register it via the layer bus.
export const IsochroneRequestSchema = z.object({
  origin: LatLonSchema,
  mode: RouteModeSchema.optional(),
  contoursSeconds: z.array(z.number().int().positive()).min(1),
});
export type IsochroneRequest = z.infer<typeof IsochroneRequestSchema>;

export const IsochroneFeatureSchema = z.object({
  type: z.literal("Feature"),
  geometry: z.object({
    type: z.enum(["Polygon", "MultiPolygon"]),
    coordinates: z.unknown(),
  }),
  properties: z.record(z.unknown()).nullable().optional(),
});

export const IsochroneResponseSchema = z.object({
  type: z.literal("FeatureCollection"),
  features: z.array(IsochroneFeatureSchema),
});
export type IsochroneResponse = z.infer<typeof IsochroneResponseSchema>;

export const RegionStateSchema = z.enum([
  "not_installed",
  "downloading",
  "processing_tiles",
  "processing_routing",
  "processing_geocoding",
  "processing_poi",
  "ready",
  "updating",
  "failed",
  "archived",
]);
export type RegionStateValue = z.infer<typeof RegionStateSchema>;

export const RegionScheduleSchema = z.string();
export type RegionSchedule = z.infer<typeof RegionScheduleSchema>;

export const RegionSchema = z.object({
  name: z.string(),
  displayName: z.string(),
  parent: z.string().nullable().optional(),
  sourceUrl: z.string(),
  sourcePbfBytes: z.number().nullable().optional(),
  sourcePbfSha256: z.string().nullable().optional(),
  bbox: BBoxSchema.nullable().optional(),
  state: RegionStateSchema,
  stateDetail: z.string().nullable().optional(),
  lastError: z.string().nullable().optional(),
  installedAt: z.string().nullable().optional(),
  lastUpdatedAt: z.string().nullable().optional(),
  nextUpdateAt: z.string().nullable().optional(),
  schedule: RegionScheduleSchema.nullable().optional(),
  diskBytes: z.number().nullable().optional(),
  activeJobId: z.string().nullable().optional(),
});
export type Region = z.infer<typeof RegionSchema>;

export const RegionsListResponseSchema = z.object({
  regions: z.array(RegionSchema),
});
export type RegionsListResponse = z.infer<typeof RegionsListResponseSchema>;

// openapi RegionCatalogEntry is recursive, so use a z.lazy schema.
export interface RegionCatalogEntry {
  name: string;
  displayName: string;
  kind: "continent" | "country" | "subregion";
  parent?: string | null;
  sourceUrl: string;
  sourcePbfBytes?: number | null;
  iso3166_1?: string | null;
  estimatedBuildBytes?: number | null;
  children?: RegionCatalogEntry[];
}

export const RegionCatalogEntrySchema: z.ZodType<RegionCatalogEntry> = z.lazy(
  () =>
    z.object({
      name: z.string(),
      displayName: z.string(),
      kind: z.enum(["continent", "country", "subregion"]),
      parent: z.string().nullable().optional(),
      sourceUrl: z.string(),
      sourcePbfBytes: z.number().nullable().optional(),
      iso3166_1: z.string().nullable().optional(),
      estimatedBuildBytes: z.number().nullable().optional(),
      children: z.array(RegionCatalogEntrySchema).optional(),
    }),
);

export const RegionCatalogResponseSchema = z.object({
  catalog: z.array(RegionCatalogEntrySchema),
  fetchedAt: z.string().optional(),
});
export type RegionCatalogResponse = z.infer<typeof RegionCatalogResponseSchema>;

export const JobStateSchema = z.enum([
  "queued",
  "running",
  "succeeded",
  "failed",
  "cancelled",
]);

export const JobKindSchema = z.enum([
  "download_pbf",
  "build_pmtiles",
  "build_valhalla",
  "build_pelias",
  "build_overture",
  "swap_region",
  "update_region",
  "archive_region",
]);

export const JobSchema = z.object({
  id: z.string(),
  kind: JobKindSchema,
  region: z.string().nullable().optional(),
  state: JobStateSchema,
  progress: z.number().min(0).max(1).nullable().optional(),
  message: z.string().nullable().optional(),
  startedAt: z.string().nullable().optional(),
  finishedAt: z.string().nullable().optional(),
  error: z.string().nullable().optional(),
  parentJobId: z.string().nullable().optional(),
});
export type Job = z.infer<typeof JobSchema>;

// POST /api/regions — returns { region, jobId }
export const RegionInstallResponseSchema = z.object({
  region: RegionSchema,
  jobId: z.string(),
});
export type RegionInstallResponse = z.infer<typeof RegionInstallResponseSchema>;

// POST /api/regions/{name}/update — returns { jobId }
export const RegionUpdateResponseSchema = z.object({
  jobId: z.string(),
});
export type RegionUpdateResponse = z.infer<typeof RegionUpdateResponseSchema>;

// DELETE /api/regions/{name} — returns { region }
export const RegionDeleteResponseSchema = z.object({
  region: RegionSchema,
});
export type RegionDeleteResponse = z.infer<typeof RegionDeleteResponseSchema>;

// POST /api/regions/{name}/activate — returns { region, activeRegion }
//
// Sets `routing.activeRegion` and writes the pointer file Valhalla polls
// for live region switching. Response carries the canonical name of the
// region now serving routing.
export const RegionActivateResponseSchema = z.object({
  region: RegionSchema,
  activeRegion: z.string(),
});
export type RegionActivateResponse = z.infer<typeof RegionActivateResponseSchema>;

// WebSocket event envelope (server → client). See openapi /api/ws description.
export const WsEventTypeSchema = z.enum([
  "region.progress",
  "region.ready",
  "region.failed",
  "job.started",
  "job.progress",
  "job.complete",
  "job.failed",
]);
export type WsEventType = z.infer<typeof WsEventTypeSchema>;

export const WsRegionEventSchema = z.object({
  type: z.enum(["region.progress", "region.ready", "region.failed"]),
  data: RegionSchema,
});
export const WsJobEventSchema = z.object({
  type: z.enum(["job.started", "job.progress", "job.complete", "job.failed"]),
  data: JobSchema,
});
export const WsEventSchema = z.union([WsRegionEventSchema, WsJobEventSchema]);
export type WsEvent = z.infer<typeof WsEventSchema>;

// --- POIs ----------------------------------------------------------
// Mirrors components.schemas.Poi in contracts/openapi.yaml.
export const PoiSchema = z.object({
  id: z.string(),
  label: z.string(),
  category: z.string().nullable().optional(),
  center: LatLonSchema,
  tags: z.record(z.string()).optional(),
  source: z.enum(["overture", "osm"]).optional(),
  region: z.string().nullable().optional(),
});
export type Poi = z.infer<typeof PoiSchema>;

// GET /api/pois/{id} — returns a single Poi.
export const PoiResponseSchema = PoiSchema;
export type PoiResponse = z.infer<typeof PoiResponseSchema>;

// GET /api/pois?bbox=&q=&category=
export const PoiListResponseSchema = z.object({
  pois: z.array(PoiSchema),
  traceId: TraceIdSchema.optional(),
});
export type PoiListResponse = z.infer<typeof PoiListResponseSchema>;

// --- Settings ------------------------------------------------------
// Mirrors components.schemas.SettingsTree + SettingsSchemaNode in
// contracts/openapi.yaml. The UI extensions (key, uiGroup, unit, step,
// readOnly, itemType, pattern) are additive under the tree's
// additionalProperties:true rule — the server emits them; the zod
// schema declares them as optional so the UI can rely on their type.
export const SettingsNodeTypeSchema = z.enum([
  "object",
  "string",
  "integer",
  "number",
  "boolean",
  "enum",
  "array",
]);
export type SettingsNodeType = z.infer<typeof SettingsNodeTypeSchema>;

export const SettingsSchemaNodeSchema = z.object({
  key: z.string(),
  type: SettingsNodeTypeSchema,
  description: z.string().optional(),
  default: z.unknown(),
  enumValues: z.array(z.unknown()).optional(),
  minimum: z.number().optional(),
  maximum: z.number().optional(),
  pattern: z.string().optional(),
  uiGroup: z.string(),
  unit: z.string().optional(),
  itemType: SettingsNodeTypeSchema.optional(),
  step: z.number().optional(),
  readOnly: z.boolean().optional(),
});
export type SettingsSchemaNode = z.infer<typeof SettingsSchemaNodeSchema>;

export const SettingsSchemaResponseSchema = z.object({
  version: z.number().int(),
  nodes: z.array(SettingsSchemaNodeSchema),
});
export type SettingsSchemaResponse = z.infer<
  typeof SettingsSchemaResponseSchema
>;

// SettingsTree is free-form per openapi. The UI carries it as a generic
// record; individual pieces are validated via the schema node list.
export const SettingsTreeSchema = z.record(z.unknown());
export type SettingsTree = z.infer<typeof SettingsTreeSchema>;

// --- Share / short links -------------------------------------------
// Mirrors components.schemas.ShortLink in contracts/openapi.yaml:
//   required: [code, url, createdAt]
//   optional: lastHitAt (nullable), hitCount
export const ShortLinkSchema = z.object({
  code: z.string(),
  url: z.string(),
  createdAt: z.string(),
  lastHitAt: z.string().nullable().optional(),
  hitCount: z.number().int().optional(),
});
export type ShortLink = z.infer<typeof ShortLinkSchema>;

// --- Auth ----------------------------------------------------------
// Mirrors components.schemas.AuthUser + AuthMeResponse in openapi.yaml.
export const AuthUserSchema = z.object({
  id: z.number().int(),
  username: z.string().min(1),
  role: z.enum(["admin", "viewer"]),
  createdAt: z.string().optional(),
  lastLoginAt: z.string().nullable().optional(),
  disabled: z.boolean().optional(),
});
export type AuthUser = z.infer<typeof AuthUserSchema>;

export const AuthMeResponseSchema = z.object({
  user: AuthUserSchema,
});
export type AuthMeResponse = z.infer<typeof AuthMeResponseSchema>;

export const AuthOkResponseSchema = z.object({
  ok: z.boolean(),
});
export type AuthOkResponse = z.infer<typeof AuthOkResponseSchema>;
