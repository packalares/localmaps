/**
 * Encodes a ShareableState into a hash + query string that can be appended
 * to the app's origin+pathname to produce a deep link.
 *
 * Hash format is inherited verbatim from `lib/url-state.ts` (backward
 * compatible with Phase 3 URLs). Everything else rides in the query
 * string:
 *
 *   ?r=<region>
 *    &route=<mode>|<lng,lat;lng,lat[;...]>|<flags>
 *    &poi=<id>
 *    &tab=<search|directions|place|saved>
 *    &q=<search query>
 *
 * Route flags are a compact letter-set: `h` (noHighways), `t` (noTolls),
 * `f` (noFerries). Missing = allowed.
 *
 * The encoder is pure; it never touches window/document.
 */

import { formatHash } from "@/lib/url-state";
import { isCanonicalRegionKey } from "@/lib/map/region-key";
import type { LeftRailTab } from "@/lib/state/map";
import type { RouteMode } from "@/lib/api/schemas";
import {
  URL_BUDGET_CHARS,
  type EncodedState,
  type ShareRoute,
  type ShareableState,
} from "./types";

const COORD_PRECISION = 5;
const VALID_MODES: readonly RouteMode[] = [
  "auto",
  "bicycle",
  "pedestrian",
  "truck",
];
const VALID_TABS: readonly LeftRailTab[] = [
  "search",
  "results",
  "directions",
  "place",
  "saved",
];

function fmt(n: number): string {
  // toFixed then strip trailing zeros; keeps the URL compact while being
  // precise enough for practical pin placement (~1.1 m at the equator).
  return Number(n.toFixed(COORD_PRECISION)).toString();
}

/** Serialise a `ShareRoute` to the compact `mode|lng,lat;...|flags` form. */
export function encodeRoute(route: ShareRoute): string | null {
  if (!VALID_MODES.includes(route.mode)) return null;
  if (!Array.isArray(route.waypoints) || route.waypoints.length === 0) {
    return null;
  }
  const coords: string[] = [];
  for (const wp of route.waypoints) {
    if (
      !Number.isFinite(wp.lng) ||
      !Number.isFinite(wp.lat) ||
      wp.lng < -180 ||
      wp.lng > 180 ||
      wp.lat < -90 ||
      wp.lat > 90
    ) {
      return null;
    }
    coords.push(`${fmt(wp.lng)},${fmt(wp.lat)}`);
  }
  let flags = "";
  if (route.options.avoidHighways) flags += "h";
  if (route.options.avoidTolls) flags += "t";
  if (route.options.avoidFerries) flags += "f";
  return `${route.mode}|${coords.join(";")}|${flags}`;
}

/**
 * Encode a full ShareableState. Deterministic: repeated calls with the same
 * input return the same output. Pure and SSR-safe.
 */
export function encodeState(state: ShareableState): EncodedState {
  const hash = formatHash(state.viewport);

  const params = new URLSearchParams();
  if (state.activeRegion && isCanonicalRegionKey(state.activeRegion)) {
    params.set("r", state.activeRegion);
  }
  if (state.leftRailTab && VALID_TABS.includes(state.leftRailTab)) {
    // `search` is the default — omit it to keep short links short.
    if (state.leftRailTab !== "search") {
      params.set("tab", state.leftRailTab);
    }
  }
  if (state.selectedPoiId && state.selectedPoiId.length > 0) {
    params.set("poi", state.selectedPoiId);
  }
  if (state.searchQuery && state.searchQuery.trim().length > 0) {
    params.set("q", state.searchQuery.trim());
  }
  if (state.route) {
    const encoded = encodeRoute(state.route);
    if (encoded) params.set("route", encoded);
  }

  const qs = params.toString();
  const query = qs.length > 0 ? `?${qs}` : "";
  // Full length budget counts both parts (plus the `#` separator).
  const length = query.length + (hash.length > 0 ? 1 + hash.length : 0);
  return {
    hash,
    query,
    length,
    overBudget: length > URL_BUDGET_CHARS,
  };
}

/**
 * Convenience: build an absolute URL from an origin + pathname and the
 * encoded pieces. Never throws; callers that need to detect the
 * overBudget signal should inspect the EncodedState instead.
 */
export function buildShareUrl(
  origin: string,
  pathname: string,
  encoded: EncodedState,
): string {
  const hashFragment = encoded.hash.length > 0 ? `#${encoded.hash}` : "";
  return `${origin}${pathname}${encoded.query}${hashFragment}`;
}
