/**
 * Decodes a URL (absolute or relative) into a `DecodedState`. Every field
 * is best-effort; malformed pieces are discarded silently so a partial URL
 * still restores what it can.
 *
 * Backward compatibility: a plain `#zoom/lat/lon[/bearing/pitch]` URL
 * (Phase 3) decodes into just `{ viewport }`.
 *
 * Pure; safe under SSR (callers pass `url` explicitly).
 */

import { parseHash } from "@/lib/url-state";
import { isCanonicalRegionKey } from "@/lib/map/region-key";
import type { LeftRailTab } from "@/lib/state/map";
import type { RouteMode } from "@/lib/api/schemas";
import type {
  DecodedState,
  ShareRoute,
  ShareRouteOptions,
  ShareWaypoint,
} from "./types";

const VALID_MODES: readonly RouteMode[] = [
  "auto",
  "bicycle",
  "pedestrian",
  "truck",
];
const VALID_TABS: readonly LeftRailTab[] = [
  "search",
  "directions",
  "saved",
  "recents",
  "categoryResults",
];

function parseUrl(input: string | URL): URL | null {
  if (input instanceof URL) return input;
  if (typeof input !== "string") return null;
  try {
    // Absolute URL first.
    return new URL(input);
  } catch {
    // Only fall back to a synthetic origin for things that look like a
    // path / query / hash; the URL constructor is otherwise too tolerant
    // (it accepts arbitrary garbage as a relative path).
    if (input.startsWith("/") || input.startsWith("?") || input.startsWith("#")) {
      try {
        return new URL(input, "http://localmaps.local");
      } catch {
        return null;
      }
    }
    return null;
  }
}

/** Decode a `route=` value. Returns null when unrecoverable. */
export function decodeRoute(raw: string): ShareRoute | null {
  if (!raw) return null;
  const parts = raw.split("|");
  if (parts.length < 2) return null;
  const [modeRaw, coordsRaw, flagsRaw = ""] = parts;
  if (!VALID_MODES.includes(modeRaw as RouteMode)) return null;

  const waypoints: ShareWaypoint[] = [];
  for (const pair of coordsRaw.split(";")) {
    if (!pair) continue;
    const [lngStr, latStr] = pair.split(",");
    const lng = Number.parseFloat(lngStr ?? "");
    const lat = Number.parseFloat(latStr ?? "");
    if (
      !Number.isFinite(lng) ||
      !Number.isFinite(lat) ||
      lng < -180 ||
      lng > 180 ||
      lat < -90 ||
      lat > 90
    ) {
      return null;
    }
    waypoints.push({ lng, lat });
  }
  if (waypoints.length === 0) return null;

  const options: ShareRouteOptions = {
    avoidHighways: flagsRaw.includes("h"),
    avoidTolls: flagsRaw.includes("t"),
    avoidFerries: flagsRaw.includes("f"),
  };
  return { mode: modeRaw as RouteMode, waypoints, options };
}

/**
 * Decode any URL-like input into a partial ShareableState. Returns null
 * only when the URL itself is unparsable. An otherwise empty URL returns
 * an empty object — callers treat that as "no state to restore".
 */
export function decodeURL(input: string | URL): DecodedState | null {
  const url = parseUrl(input);
  if (!url) return null;
  const out: DecodedState = {};

  if (url.hash && url.hash.length > 1) {
    const vp = parseHash(url.hash);
    if (vp) out.viewport = vp;
  }

  // Share-button compatibility: the bottom info card's "Copy link"
  // affordance writes `?lat=&lon=&zoom=&place=`. Decode it into the
  // same DecodedState shape the canonical encoder produces so the
  // single restore path (`applyDecoded`) handles both. The hash form
  // (`#zoom/lat/lon`) wins when both are present.
  if (!out.viewport) {
    const flatVp = parseFlatViewportParams(url.searchParams);
    if (flatVp) out.viewport = flatVp;
  }

  const region = url.searchParams.get("r");
  if (region && isCanonicalRegionKey(region)) {
    out.activeRegion = region;
  }

  const tab = url.searchParams.get("tab");
  if (tab && VALID_TABS.includes(tab as LeftRailTab)) {
    out.leftRailTab = tab as LeftRailTab;
  }

  // `poi` (canonical) and `place` (share-button) both round-trip a
  // POI id. Prefer `poi` when both are set so the canonical encoder
  // takes precedence on re-shares.
  const poi = url.searchParams.get("poi") ?? url.searchParams.get("place");
  if (poi && poi.length > 0 && poi.length < 512) {
    out.selectedPoiId = poi;
  }

  const q = url.searchParams.get("q");
  if (q && q.length > 0 && q.length < 512) {
    out.searchQuery = q;
  }

  const routeRaw = url.searchParams.get("route");
  if (routeRaw) {
    const route = decodeRoute(routeRaw);
    if (route) out.route = route;
  }

  return out;
}

/** Parse the `?lat=&lon=&zoom=` triple emitted by the share-button. */
function parseFlatViewportParams(
  params: URLSearchParams,
): import("@/lib/url-state").MapViewport | null {
  const latRaw = params.get("lat");
  const lonRaw = params.get("lon");
  const zoomRaw = params.get("zoom");
  if (!latRaw || !lonRaw) return null;
  const lat = Number.parseFloat(latRaw);
  const lon = Number.parseFloat(lonRaw);
  const zoom = zoomRaw ? Number.parseFloat(zoomRaw) : 15;
  if (
    !Number.isFinite(lat) ||
    !Number.isFinite(lon) ||
    !Number.isFinite(zoom) ||
    lat < -90 ||
    lat > 90 ||
    lon < -180 ||
    lon > 180 ||
    zoom < 0 ||
    zoom > 24
  ) {
    return null;
  }
  return { lat, lon, zoom, bearing: 0, pitch: 0 };
}
