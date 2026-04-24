"use client";

import { useEffect, useMemo } from "react";
import {
  useMutation,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { apiRequest } from "./client";
import { apiUrl } from "@/lib/env";
import { useMapStore } from "@/lib/state/map";
import {
  AuthMeResponseSchema,
  type AuthMeResponse,
  type AuthUser,
  AuthOkResponseSchema,
  type AuthOkResponse,
  GeocodeAutocompleteResponseSchema,
  type GeocodeAutocompleteResponse,
  GeocodeReverseResponseSchema,
  type GeocodeReverseResponse,
  GeocodeSearchResponseSchema,
  type GeocodeSearchResponse,
  JobSchema,
  type Job,
  PoiListResponseSchema,
  type PoiListResponse,
  PoiResponseSchema,
  type PoiResponse,
  type Poi,
  RegionCatalogResponseSchema,
  type RegionCatalogResponse,
  RegionDeleteResponseSchema,
  type RegionDeleteResponse,
  RegionInstallResponseSchema,
  type RegionInstallResponse,
  RegionSchema,
  type Region,
  RegionsListResponseSchema,
  type RegionsListResponse,
  RegionUpdateResponseSchema,
  type RegionUpdateResponse,
  IsochroneRequestSchema,
  IsochroneResponseSchema,
  type IsochroneRequest,
  type IsochroneResponse,
  RouteRequestSchema,
  RouteResponseSchema,
  type RouteRequest,
  type RouteResponse,
  SettingsSchemaResponseSchema,
  type SettingsSchemaResponse,
  SettingsTreeSchema,
  type SettingsTree,
  ShortLinkSchema,
  type ShortLink,
} from "./schemas";

/**
 * Thin hooks over the gateway endpoints defined in
 * `contracts/openapi.yaml`. Every call validates its response via zod.
 *
 * See `docs/03-contracts.md` for the route catalogue. No hook may call
 * an endpoint not in the OpenAPI document.
 */

// GET /api/geocode/autocomplete
export interface UseGeocodeAutocompleteArgs {
  q: string;
  focus?: { lat: number; lon: number };
  limit?: number;
  lang?: string;
  /** If true (the default) the query is disabled when q is empty. */
  enabled?: boolean;
}

export function useGeocodeAutocomplete(
  args: UseGeocodeAutocompleteArgs,
): ReturnType<typeof useQuery<GeocodeAutocompleteResponse>> {
  const query: Record<string, string | number | undefined> = {
    q: args.q,
    limit: args.limit,
    lang: args.lang,
  };
  if (args.focus) {
    query["focus.lat"] = args.focus.lat;
    query["focus.lon"] = args.focus.lon;
  }

  const enabled = (args.enabled ?? true) && args.q.trim().length > 0;

  return useQuery({
    queryKey: ["geocode", "autocomplete", args] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/geocode/autocomplete",
        query,
        schema: GeocodeAutocompleteResponseSchema,
        signal,
      }),
    staleTime: 30_000,
  });
}

// GET /api/geocode/search — full-text search (sentence-level queries).
export interface UseGeocodeSearchArgs {
  q: string;
  focus?: { lat: number; lon: number };
  limit?: number;
  enabled?: boolean;
}

export function useGeocodeSearch(
  args: UseGeocodeSearchArgs,
): ReturnType<typeof useQuery<GeocodeSearchResponse>> {
  const query: Record<string, string | number | undefined> = {
    q: args.q,
    limit: args.limit,
  };
  if (args.focus) {
    query["focus.lat"] = args.focus.lat;
    query["focus.lon"] = args.focus.lon;
  }
  const enabled = (args.enabled ?? true) && args.q.trim().length > 0;

  return useQuery({
    queryKey: ["geocode", "search", args] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/geocode/search",
        query,
        schema: GeocodeSearchResponseSchema,
        signal,
      }),
    staleTime: 30_000,
  });
}

// GET /api/geocode/reverse — nearest address to a coordinate.
export interface UseReverseGeocodeArgs {
  lngLat: { lng: number; lat: number } | null;
  enabled?: boolean;
}

export function useReverseGeocode(
  args: UseReverseGeocodeArgs,
): ReturnType<typeof useQuery<GeocodeReverseResponse>> {
  const enabled = (args.enabled ?? true) && !!args.lngLat;
  return useQuery({
    queryKey: [
      "geocode",
      "reverse",
      args.lngLat
        ? `${args.lngLat.lat.toFixed(6)},${args.lngLat.lng.toFixed(6)}`
        : "",
    ] as const,
    enabled,
    queryFn: ({ signal }) => {
      const ll = args.lngLat!;
      return apiRequest({
        path: "/api/geocode/reverse",
        query: { lat: ll.lat, lon: ll.lng },
        schema: GeocodeReverseResponseSchema,
        signal,
      });
    },
    staleTime: 60_000,
  });
}

// POST /api/route
//
// Returned as a mutation so callers can fire-and-forget on waypoint
// changes. The `mutationKey` is stable (`["route"]`) so React-Query
// dedupes concurrent fires while still surfacing the latest result —
// which combined with `useRouteSync`'s effect key gives scoped
// invalidation: we only refetch when a waypoint, mode, or avoid
// option actually changed.
export function useRoute(): ReturnType<
  typeof useMutation<RouteResponse, Error, RouteRequest>
> {
  return useMutation({
    mutationKey: ["route"] as const,
    mutationFn: (req: RouteRequest) => {
      const body = RouteRequestSchema.parse(req);
      return apiRequest({
        method: "POST",
        path: "/api/route",
        body,
        schema: RouteResponseSchema,
      });
    },
  });
}

// POST /api/isochrone — reachable-in-time polygons.
//
// Exposed as a mutation so the Tools panel can trigger a request on an
// explicit "Render" click. Response is a GeoJSON FeatureCollection with
// one polygon feature per entry in `contoursSeconds`.
export function useIsochrone(): ReturnType<
  typeof useMutation<IsochroneResponse, Error, IsochroneRequest>
> {
  return useMutation({
    mutationKey: ["isochrone"] as const,
    mutationFn: (req: IsochroneRequest) => {
      const body = IsochroneRequestSchema.parse(req);
      return apiRequest({
        method: "POST",
        path: "/api/isochrone",
        body,
        schema: IsochroneResponseSchema,
      });
    },
  });
}

// GET /api/regions/catalog
export function useRegionCatalog(
  options: { force?: boolean; enabled?: boolean } = {},
): ReturnType<typeof useQuery<RegionCatalogResponse>> {
  return useQuery({
    queryKey: ["regions", "catalog", options.force ?? false] as const,
    enabled: options.enabled ?? true,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/regions/catalog",
        query: options.force ? { force: true } : undefined,
        schema: RegionCatalogResponseSchema,
        signal,
      }),
    staleTime: 5 * 60_000,
  });
}

// GET /api/pois/{id}
export function usePoi(
  id: string | null | undefined,
  options: { enabled?: boolean } = {},
): ReturnType<typeof useQuery<PoiResponse>> {
  const enabled = (options.enabled ?? true) && !!id && id.length > 0;
  return useQuery({
    queryKey: ["pois", "byId", id ?? ""] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: `/api/pois/${encodeURIComponent(id as string)}`,
        schema: PoiResponseSchema,
        signal,
      }),
    staleTime: 60_000,
  });
}

/**
 * Nearest-POI lookup. The OpenAPI does not expose a dedicated "nearest"
 * endpoint; we compose a small bbox around the point (radius in metres
 * converted to degrees) and pick the closest result. If the gateway
 * later adds `/api/pois/nearest`, swap this body over.
 */
export interface UseNearestPoiArgs {
  lngLat: { lng: number; lat: number } | null;
  /** Radius in metres. Default 50. */
  radius?: number;
  enabled?: boolean;
}

// Approximate metres-per-degree at the equator; we widen the box by 1.5×
// to compensate for longitude shrinkage at higher latitudes. Good enough
// for a 50 m click tolerance.
const METRES_PER_DEG = 111_320;

export function poiBboxAround(
  lng: number,
  lat: number,
  radiusMetres: number,
): string {
  const degLat = radiusMetres / METRES_PER_DEG;
  const cos = Math.cos((lat * Math.PI) / 180) || 1;
  const degLon = degLat / Math.max(0.01, cos);
  const minLon = lng - degLon;
  const maxLon = lng + degLon;
  const minLat = lat - degLat;
  const maxLat = lat + degLat;
  return `${minLon},${minLat},${maxLon},${maxLat}`;
}

function haversineMetres(
  a: { lng: number; lat: number },
  b: { lng: number; lat: number },
): number {
  const R = 6_371_000;
  const toRad = (x: number) => (x * Math.PI) / 180;
  const dLat = toRad(b.lat - a.lat);
  const dLon = toRad(b.lng - a.lng);
  const lat1 = toRad(a.lat);
  const lat2 = toRad(b.lat);
  const s =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(lat1) * Math.cos(lat2) * Math.sin(dLon / 2) ** 2;
  return 2 * R * Math.asin(Math.sqrt(s));
}

export function useNearestPoi(
  args: UseNearestPoiArgs,
): ReturnType<typeof useQuery<Poi | null>> {
  const { lngLat, radius = 50, enabled } = args;
  const isEnabled = (enabled ?? true) && !!lngLat;

  return useQuery({
    queryKey: [
      "pois",
      "nearest",
      lngLat ? `${lngLat.lng.toFixed(6)},${lngLat.lat.toFixed(6)}` : "",
      radius,
    ] as const,
    enabled: isEnabled,
    queryFn: async ({ signal }) => {
      if (!lngLat) return null;
      const bbox = poiBboxAround(lngLat.lng, lngLat.lat, radius);
      const res = await apiRequest({
        path: "/api/pois",
        query: { bbox, limit: 25 },
        schema: PoiListResponseSchema,
        signal,
      });
      if (!res.pois.length) return null;
      // Pick closest by haversine; keeps results monotonic regardless
      // of gateway ordering.
      let best: Poi | null = null;
      let bestDist = Infinity;
      for (const p of res.pois) {
        const d = haversineMetres(lngLat, {
          lng: p.center.lon,
          lat: p.center.lat,
        });
        if (d < bestDist) {
          bestDist = d;
          best = p;
        }
      }
      return bestDist <= radius ? best : best; // Return even if outside radius; caller decides.
    },
    staleTime: 30_000,
  });
}

/**
 * Text search over POIs. Backed by `GET /api/pois?q=...` (optional
 * bbox narrow). If the user later wires a dedicated `/api/pois/search`
 * endpoint, update the path here.
 */
export interface UsePoiSearchArgs {
  q: string;
  bbox?: string;
  category?: string;
  limit?: number;
  enabled?: boolean;
}

export function usePoiSearch(
  args: UsePoiSearchArgs,
): ReturnType<typeof useQuery<PoiListResponse>> {
  const enabled = (args.enabled ?? true) && args.q.trim().length > 0;
  return useQuery({
    queryKey: ["pois", "search", args] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/pois",
        query: {
          q: args.q,
          bbox: args.bbox,
          category: args.category,
          limit: args.limit,
        },
        schema: PoiListResponseSchema,
        signal,
      }),
    staleTime: 30_000,
  });
}

/** List of installed + in-progress regions on the server. Polls every 15s. */
export function useRegions(): ReturnType<
  typeof useQuery<RegionsListResponse>
> {
  return useQuery({
    queryKey: ["regions", "list"] as const,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/regions",
        schema: RegionsListResponseSchema,
        signal,
      }),
    staleTime: 15_000,
    refetchInterval: 15_000,
  });
}

/**
 * Polls `/api/regions` every 15s and seeds the Zustand `installedRegions`
 * so UI chrome (region switcher, search biasing, etc.) can read from the
 * store without each caller wiring up its own query. Returns the raw
 * query result too, for components that want loading/error state.
 */
export function useRegionsSync(): ReturnType<
  typeof useQuery<RegionsListResponse>
> {
  const query = useRegions();
  const setInstalledRegions = useMapStore((s) => s.setInstalledRegions);

  useEffect(() => {
    if (query.data) {
      setInstalledRegions(query.data.regions);
    }
  }, [query.data, setInstalledRegions]);

  return query;
}

/**
 * Memoised MapLibre style URL for a given region + theme. When `region`
 * is non-null it is forwarded as `?region=<canonical-key>` so the gateway
 * can resolve the correct pmtiles archive; the style name itself is one
 * of the three `/api/styles/{name}.json` variants declared in
 * `contracts/openapi.yaml`.
 *
 * `language` threads the operator's `map.language` preference through
 * to the style endpoint as `?lang=<ISO>`. The value is only appended
 * when non-empty and non-`"default"` (see `docs/07-config-schema.md`).
 * The gateway is expected to accept it best-effort; we do not fail
 * client-side when the server ignores it.
 */
export function useStyleUrl(
  region: string | null,
  theme: "light" | "dark" | "print" = "light",
  language?: string | null,
): string {
  return useMemo(() => {
    const base = apiUrl(`/api/styles/${theme}.json`);
    const params = new URLSearchParams();
    if (region) params.set("region", region);
    if (language && language !== "default") params.set("lang", language);
    const qs = params.toString();
    return qs ? `${base}?${qs}` : base;
  }, [region, theme, language]);
}

// GET /api/regions/{name} — single region state.
export function useRegion(
  name: string | null | undefined,
  options: { enabled?: boolean } = {},
): ReturnType<typeof useQuery<Region>> {
  const enabled = (options.enabled ?? true) && !!name && name.length > 0;
  return useQuery({
    queryKey: ["regions", "byName", name ?? ""] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: `/api/regions/${encodeURIComponent(name as string)}`,
        schema: RegionSchema,
        signal,
      }),
    staleTime: 5_000,
  });
}

// GET /api/jobs/{jobId}
export function useJob(
  jobId: string | null | undefined,
  options: { enabled?: boolean; refetchIntervalMs?: number } = {},
): ReturnType<typeof useQuery<Job>> {
  const enabled = (options.enabled ?? true) && !!jobId && jobId.length > 0;
  return useQuery({
    queryKey: ["jobs", "byId", jobId ?? ""] as const,
    enabled,
    queryFn: ({ signal }) =>
      apiRequest({
        path: `/api/jobs/${encodeURIComponent(jobId as string)}`,
        schema: JobSchema,
        signal,
      }),
    refetchInterval: options.refetchIntervalMs,
    staleTime: 5_000,
  });
}

// POST /api/regions — install a region. Body { name, schedule? }.
export function useInstallRegion(): ReturnType<
  typeof useMutation<
    RegionInstallResponse,
    Error,
    { name: string; schedule?: string }
  >
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["regions", "install"] as const,
    mutationFn: (req) =>
      apiRequest({
        method: "POST",
        path: "/api/regions",
        body: req,
        schema: RegionInstallResponseSchema,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["regions", "list"] });
      qc.invalidateQueries({ queryKey: ["regions", "catalog"] });
    },
  });
}

// POST /api/regions/{name}/update — re-download + rebuild.
export function useUpdateRegionNow(): ReturnType<
  typeof useMutation<RegionUpdateResponse, Error, { name: string }>
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["regions", "update"] as const,
    mutationFn: ({ name }) =>
      apiRequest({
        method: "POST",
        path: `/api/regions/${encodeURIComponent(name)}/update`,
        schema: RegionUpdateResponseSchema,
      }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["regions", "list"] });
      qc.invalidateQueries({ queryKey: ["regions", "byName", vars.name] });
    },
  });
}

// DELETE /api/regions/{name}
export function useDeleteRegion(): ReturnType<
  typeof useMutation<RegionDeleteResponse, Error, { name: string }>
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["regions", "delete"] as const,
    mutationFn: ({ name }) =>
      apiRequest({
        method: "DELETE",
        path: `/api/regions/${encodeURIComponent(name)}`,
        schema: RegionDeleteResponseSchema,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ["regions", "list"] });
      qc.invalidateQueries({ queryKey: ["regions", "catalog"] });
    },
  });
}

// POST /api/links — create a short link from the current long URL.
//
// Returns the full `ShortLink` envelope per contracts/openapi.yaml. The
// UI composes `shortUrl` by joining `code` to the window origin at call
// time — the server stores a relative URL, not a pre-baked absolute.
export function useCreateShortLink(): ReturnType<
  typeof useMutation<ShortLink, Error, { url: string }>
> {
  return useMutation({
    mutationKey: ["links", "create"] as const,
    mutationFn: ({ url }) =>
      apiRequest({
        method: "POST",
        path: "/api/links",
        body: { url },
        schema: ShortLinkSchema,
      }),
  });
}

// GET /api/settings — returns the current settings tree (admin only).
export function useSettings(
  options: { enabled?: boolean } = {},
): ReturnType<typeof useQuery<SettingsTree>> {
  return useQuery({
    queryKey: ["settings", "tree"] as const,
    enabled: options.enabled ?? true,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/settings",
        schema: SettingsTreeSchema,
        signal,
      }),
    staleTime: 30_000,
  });
}

// GET /api/settings/schema — anonymous; returns the schema node list.
export function useSettingsSchema(
  options: { enabled?: boolean } = {},
): ReturnType<typeof useQuery<SettingsSchemaResponse>> {
  return useQuery({
    queryKey: ["settings", "schema"] as const,
    enabled: options.enabled ?? true,
    queryFn: ({ signal }) =>
      apiRequest({
        path: "/api/settings/schema",
        schema: SettingsSchemaResponseSchema,
        signal,
      }),
    staleTime: 5 * 60_000,
  });
}

// PATCH /api/settings — sends the diff (flat or nested). The server
// returns the updated tree.
export function useSaveSettings(): ReturnType<
  typeof useMutation<SettingsTree, Error, Record<string, unknown>>
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["settings", "save"] as const,
    mutationFn: (patch: Record<string, unknown>) =>
      apiRequest({
        method: "PATCH",
        path: "/api/settings",
        body: patch,
        schema: SettingsTreeSchema,
      }),
    onSuccess: (tree) => {
      qc.setQueryData(["settings", "tree"], tree);
    },
  });
}

// PUT /api/regions/{name}/schedule — set update cadence.
export function useSetRegionSchedule(): ReturnType<
  typeof useMutation<Region, Error, { name: string; schedule: string }>
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["regions", "schedule"] as const,
    mutationFn: ({ name, schedule }) =>
      apiRequest({
        method: "PUT",
        path: `/api/regions/${encodeURIComponent(name)}/schedule`,
        body: { schedule },
        schema: RegionSchema,
      }),
    onSuccess: (_, vars) => {
      qc.invalidateQueries({ queryKey: ["regions", "list"] });
      qc.invalidateQueries({ queryKey: ["regions", "byName", vars.name] });
    },
  });
}

// --- Auth hooks ----------------------------------------------------

/**
 * Current user. 401 responses tolerate anonymous calls (no redirect),
 * so this hook is safe to use on public pages.
 */
export function useCurrentUser(): ReturnType<
  typeof useQuery<AuthMeResponse | null>
> {
  return useQuery<AuthMeResponse | null>({
    queryKey: ["auth", "me"] as const,
    queryFn: async ({ signal }) => {
      try {
        return await apiRequest({
          path: "/api/auth/me",
          schema: AuthMeResponseSchema,
          signal,
          noAuthRedirect: true,
        });
      } catch {
        // Anonymous callers see 401 here; surface null.
        return null;
      }
    },
    staleTime: 30_000,
  });
}

export function useLogin(): ReturnType<
  typeof useMutation<
    AuthMeResponse,
    Error,
    { username: string; password: string }
  >
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["auth", "login"] as const,
    mutationFn: ({ username, password }) =>
      apiRequest({
        method: "POST",
        path: "/api/auth/login",
        body: { username, password },
        schema: AuthMeResponseSchema,
        noAuthRedirect: true,
      }),
    onSuccess: (data) => {
      qc.setQueryData(["auth", "me"], data);
    },
  });
}

export function useLogout(): ReturnType<
  typeof useMutation<AuthOkResponse, Error, void>
> {
  const qc = useQueryClient();
  return useMutation({
    mutationKey: ["auth", "logout"] as const,
    mutationFn: () =>
      apiRequest({
        method: "POST",
        path: "/api/auth/logout",
        schema: AuthOkResponseSchema,
        noAuthRedirect: true,
      }),
    onSuccess: () => {
      qc.setQueryData(["auth", "me"], null);
      qc.invalidateQueries({ queryKey: ["auth", "me"] });
    },
  });
}

// Alias re-exports for convenient consumption in component files.
export type { AuthUser };
