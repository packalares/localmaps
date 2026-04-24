/**
 * Shared types for the extended URL-state serialiser.
 *
 * Keep this in sync with:
 * - `lib/state/map.ts` (viewport, activeRegion, selectedPoi, leftRailTab,
 *   searchQuery proxy)
 * - `lib/state/directions.ts` (mode, waypoints, options)
 *
 * A `ShareableState` captures everything a deep link needs to restore; a
 * `DecodedState` is the tolerant variant used by `decodeURL` — every
 * field is optional since partial / legacy URLs are explicitly supported.
 */

import type { MapViewport } from "@/lib/url-state";
import type { LeftRailTab } from "@/lib/state/map";
import type { RouteMode } from "@/lib/api/schemas";

/** Per-route flags, one-letter each, in the URL. */
export interface ShareRouteOptions {
  avoidHighways: boolean;
  avoidTolls: boolean;
  avoidFerries: boolean;
}

/** One waypoint — only resolved coordinates survive into the URL. */
export interface ShareWaypoint {
  lng: number;
  lat: number;
}

/** Directions slice as it appears in a shareable link. */
export interface ShareRoute {
  mode: RouteMode;
  waypoints: ShareWaypoint[];
  options: ShareRouteOptions;
}

/** Full state a link intends to round-trip. */
export interface ShareableState {
  viewport: MapViewport;
  activeRegion?: string | null;
  leftRailTab?: LeftRailTab;
  selectedPoiId?: string | null;
  searchQuery?: string | null;
  route?: ShareRoute | null;
}

/** Tolerant decode result — any field can be absent. */
export interface DecodedState {
  viewport?: MapViewport;
  activeRegion?: string;
  leftRailTab?: LeftRailTab;
  selectedPoiId?: string;
  searchQuery?: string;
  route?: ShareRoute;
}

/** Upper bound the spec allows before the caller should fall back to a short link. */
export const URL_BUDGET_CHARS = 2048;

/** What `encodeState` emits. */
export interface EncodedState {
  /** Hash fragment without the leading `#`. */
  hash: string;
  /** Query string including the leading `?` (or empty when no params). */
  query: string;
  /** Total encoded length (path-less) — used to check the 2048 budget. */
  length: number;
  /** True when `length` exceeds `URL_BUDGET_CHARS`. */
  overBudget: boolean;
}
