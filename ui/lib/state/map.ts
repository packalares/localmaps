"use client";

import { create } from "zustand";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { MapViewport } from "@/lib/url-state";
import type { Poi, Region } from "@/lib/api/schemas";

/**
 * Client-only map state. Server state (routes, POIs, geocode results,
 * region list) is owned by TanStack Query — this store holds only what
 * the user's latest interaction produced in the browser plus the shared
 * references other feature modules need (MapLibre instance, pending
 * click/contextmenu, the active region key).
 *
 * Selection state for the bottom info card lives in the separate
 * `usePlaceStore` (`lib/state/place.ts`) under `selectedFeature`. That
 * is the canonical surface — there is no `selectedPoi` / `selectedResult`
 * shadow on this store.
 */

/** The currently-rendered route, if any. */
export interface ActiveRoute {
  id: string;
  mode: "auto" | "bicycle" | "pedestrian" | "truck";
  /** Encoded polyline (Valhalla / Google precision 6). */
  polyline: string;
  distanceMeters: number;
  timeSeconds: number;
}

/** Canonical representation of a map pointer event for sibling modules. */
export interface PendingPointerEvent {
  /** Geographic location under the pointer. */
  lngLat: { lng: number; lat: number };
  /** Screen-pixel point within the map canvas. */
  point: { x: number; y: number };
  /** `Date.now()` at the moment the event fired — used as a freshness key. */
  timestamp: number;
}

/** Tabs in the left rail; the search pill routes into `search`. */
export type LeftRailTab =
  | "search"
  | "directions"
  | "saved"
  | "recents"
  | "categoryResults";

/** Centralised exhaustive switch helper. Every consumer that branches
 *  on `LeftRailTab` should pass through this so adding a new tab
 *  triggers a TS error in every reader. */
export function assertNeverTab(tab: never): never {
  throw new Error(`Unhandled LeftRailTab: ${String(tab)}`);
}

/**
 * Canonical POI category keys. The server-side map style ships paired
 * `poi-<category>` (icon) and `poi-<category>-label` (label) layers using
 * exactly these suffixes — changing one of these strings without also
 * updating the style would silently break the toggle.
 */
export const POI_CATEGORIES = [
  "food",
  "shopping",
  "lodging",
  "transit",
  "healthcare",
  "services",
  "entertainment",
  "education",
  "other",
] as const;

export type PoiCategory = (typeof POI_CATEGORIES)[number];

/** localStorage key for persisted POI visibility toggles. */
export const POI_VISIBILITY_STORAGE_KEY = "localmaps.poi.visibility.v1";

/** Default state: every category visible. */
export function defaultPoiVisibility(): Record<PoiCategory, boolean> {
  const out = {} as Record<PoiCategory, boolean>;
  for (const c of POI_CATEGORIES) out[c] = true;
  return out;
}

export interface MapState {
  viewport: MapViewport;
  activeRoute: ActiveRoute | null;

  /** The live MapLibre instance, set by MapView once the style has loaded. */
  map: MapLibreMap | null;
  /** Canonical hyphenated region key (e.g. `europe-romania`) or null. */
  activeRegion: string | null;
  /** Mirror of `/api/regions` filtered to ready + in-progress regions. */
  installedRegions: Region[];

  /** Latest left-click pending consumption by search/POI panes. */
  pendingClick: PendingPointerEvent | null;
  /** Latest right-click pending consumption by the context menu. */
  pendingContextmenu: PendingPointerEvent | null;

  /** Which tab of the left rail is currently visible. */
  leftRailTab: LeftRailTab;

  /** True while the floating SearchBar's dropdown is open (input
   *  focused or showing autocomplete). Read by SelectedFeatureSync to
   *  cascade-close on map click before dropping a pin. */
  searchDropdownOpen: boolean;
  setSearchDropdownOpen: (open: boolean) => void;

  /** Increment to ask the floating SearchBar to clear its local query
   *  state. Used by the side-panel close X (which doesn't otherwise
   *  reach into SearchBar's local React state). */
  searchClearToken: number;
  requestClearSearchQuery: () => void;

  /** Per-category POI visibility toggles, persisted to localStorage. */
  poiVisibility: Record<PoiCategory, boolean>;

  /**
   * The category chip currently firing a search (pins visible, dropdown
   * open). `null` means no chip is active. Only one chip can be active
   * at a time — re-clicking the active chip clears it back to `null`.
   */
  activeCategoryChip: PoiCategory | null;
  /**
   * Search results for the active chip. Empty array means "query in
   * flight or no hits"; only meaningful while `activeCategoryChip` is
   * non-null. Consumers render these as red pins + a dropdown list.
   */
  categorySearchResults: Poi[];
  /**
   * Bbox captured at the moment a chip became active (Google-Maps "search
   * THIS area" semantics). Frozen for the chip's lifetime so subsequent
   * pan/zoom does NOT refire the query — the user explicitly clicks the
   * chip again to re-search the new viewport. Format: the same
   * `minLon,minLat,maxLon,maxLat` string the gateway expects, so the
   * hook can hand it through verbatim.
   */
  categorySearchBbox: string | null;

  setViewport: (viewport: MapViewport) => void;
  setActiveRoute: (route: ActiveRoute | null) => void;
  setMap: (map: MapLibreMap | null) => void;
  setActiveRegion: (region: string | null) => void;
  setInstalledRegions: (regions: Region[]) => void;
  setPendingClick: (event: PendingPointerEvent | null) => void;
  setPendingContextmenu: (event: PendingPointerEvent | null) => void;
  /** Clears the most recent left-click after a consumer has handled it. */
  clearPendingClick: () => void;
  /** Clears the most recent right-click after a consumer has handled it. */
  clearPendingContextmenu: () => void;
  /** Switch the left rail to the given tab (and ensure it's rendered). */
  openLeftRail: (tab: LeftRailTab) => void;
  /** Flip a single POI category between visible and hidden. */
  togglePoiCategory: (cat: PoiCategory) => void;
  /** Force one category to a specific visibility. */
  setPoiVisibility: (cat: PoiCategory, visible: boolean) => void;
  /** Replace the whole POI visibility map at once (used by shift-click solo). */
  replacePoiVisibility: (next: Record<PoiCategory, boolean>) => void;
  /**
   * Promote `cat` into the active chip slot. The HTTP fetch lives in a
   * caller-side effect (`runCategorySearch` below only flips the store;
   * `PoiSearchChips` owns the fetch so it can bind to TanStack Query's
   * lifecycle). Passing the same `cat` twice clears the active state —
   * Google-Maps chip UX.
   */
  setActiveCategoryChip: (cat: PoiCategory | null) => void;
  /** Replace the latest category-search result set. */
  setCategorySearchResults: (results: Poi[]) => void;
  /**
   * Replace the bbox snapshot used for the active chip search. Drops
   * automatically when the chip clears; callers normally just pass a
   * fresh viewport-derived string at the moment the chip is clicked.
   */
  setCategorySearchBbox: (bbox: string | null) => void;
  /**
   * Google-Maps chip action: "if active, clear; otherwise mark as
   * active". The real fetch + pin rendering is orchestrated by the
   * `PoiSearchChips` component which watches `activeCategoryChip`.
   * Optional `bbox` is captured AT THE MOMENT OF ACTIVATION and frozen
   * for the chip's lifetime — pan/zoom does not refresh the result set
   * (Change 7). Pass null to leave the bbox untouched (e.g. tests).
   */
  runCategorySearch: (cat: PoiCategory, bbox?: string | null) => void;
  /**
   * Full-reset closer for the chip results panel: clears the chip, the
   * markers, the frozen bbox, and slides the rail back to `search`.
   * Used by the panel's X button and the search-bar's clear-X while a
   * chip is active.
   */
  closeCategoryResults: () => void;
  /** Resets every field to its initial state; used on sign-out / tests. */
  clear: () => void;
}

/**
 * Read the persisted visibility map from localStorage, discarding anything
 * stale (wrong shape, unknown categories, etc.) and backfilling missing
 * categories with `true` so new categories added to the style light up by
 * default rather than appearing hidden.
 */
function loadPoiVisibility(): Record<PoiCategory, boolean> {
  const base = defaultPoiVisibility();
  if (typeof window === "undefined") return base;
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(POI_VISIBILITY_STORAGE_KEY);
  } catch {
    return base;
  }
  if (!raw) return base;
  try {
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return base;
    const record = parsed as Record<string, unknown>;
    for (const c of POI_CATEGORIES) {
      if (typeof record[c] === "boolean") base[c] = record[c] as boolean;
    }
    return base;
  } catch {
    return base;
  }
}

function persistPoiVisibility(next: Record<PoiCategory, boolean>): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(
      POI_VISIBILITY_STORAGE_KEY,
      JSON.stringify(next),
    );
  } catch {
    // localStorage unavailable (private mode, quota). Silently ignore.
  }
}

/** Initial viewport when URL hash is absent and settings provide no default. */
export const DEFAULT_VIEWPORT: MapViewport = {
  lat: 0,
  lon: 0,
  zoom: 2,
  bearing: 0,
  pitch: 0,
};

export const useMapStore = create<MapState>((set, get) => ({
  viewport: DEFAULT_VIEWPORT,
  activeRoute: null,

  map: null,
  activeRegion: null,
  installedRegions: [],

  pendingClick: null,
  pendingContextmenu: null,

  leftRailTab: "search",
  searchDropdownOpen: false,
  setSearchDropdownOpen: (open) => set({ searchDropdownOpen: open }),
  searchClearToken: 0,
  requestClearSearchQuery: () =>
    set((s) => ({ searchClearToken: s.searchClearToken + 1 })),

  poiVisibility: loadPoiVisibility(),

  activeCategoryChip: null,
  categorySearchResults: [],
  categorySearchBbox: null,

  setViewport: (viewport) => set({ viewport }),
  setActiveRoute: (activeRoute) => set({ activeRoute }),
  setMap: (map) => set({ map }),
  setActiveRegion: (activeRegion) => set({ activeRegion }),
  setInstalledRegions: (installedRegions) => set({ installedRegions }),
  setPendingClick: (pendingClick) => set({ pendingClick }),
  setPendingContextmenu: (pendingContextmenu) => set({ pendingContextmenu }),
  clearPendingClick: () => set({ pendingClick: null }),
  clearPendingContextmenu: () => set({ pendingContextmenu: null }),
  openLeftRail: (leftRailTab) => set({ leftRailTab }),
  togglePoiCategory: (cat) => {
    const current = get().poiVisibility;
    const next = { ...current, [cat]: !current[cat] };
    persistPoiVisibility(next);
    set({ poiVisibility: next });
  },
  setPoiVisibility: (cat, visible) => {
    const current = get().poiVisibility;
    if (current[cat] === visible) return;
    const next = { ...current, [cat]: visible };
    persistPoiVisibility(next);
    set({ poiVisibility: next });
  },
  replacePoiVisibility: (next) => {
    persistPoiVisibility(next);
    set({ poiVisibility: { ...next } });
  },
  setActiveCategoryChip: (cat) =>
    set(
      cat === null
        ? {
            activeCategoryChip: null,
            categorySearchResults: [],
            categorySearchBbox: null,
          }
        : { activeCategoryChip: cat },
    ),
  setCategorySearchResults: (categorySearchResults) =>
    set({ categorySearchResults }),
  setCategorySearchBbox: (categorySearchBbox) => set({ categorySearchBbox }),
  runCategorySearch: (cat, bbox) => {
    const current = get().activeCategoryChip;
    const currentTab = get().leftRailTab;
    if (current === cat) {
      // Re-clicking the active chip clears pins + closes the side panel
      // if it was showing the chip results.
      set({
        activeCategoryChip: null,
        categorySearchResults: [],
        categorySearchBbox: null,
        leftRailTab:
          currentTab === "categoryResults" ? "search" : currentTab,
      });
      return;
    }
    // Flip to the new chip; results for the previous category are
    // discarded so the caller's effect can fire a fresh request. Also
    // open the side-panel tab so the result list slides in. The bbox
    // captured here is frozen — the chip's category-search hook reads
    // from it and will NOT refire when the user pans/zooms.
    set({
      activeCategoryChip: cat,
      categorySearchResults: [],
      categorySearchBbox: bbox ?? null,
      leftRailTab: "categoryResults",
    });
  },
  closeCategoryResults: () => {
    const currentTab = get().leftRailTab;
    set({
      activeCategoryChip: null,
      categorySearchResults: [],
      categorySearchBbox: null,
      leftRailTab:
        currentTab === "categoryResults" ? "search" : currentTab,
    });
  },
  clear: () =>
    set({
      viewport: DEFAULT_VIEWPORT,
      activeRoute: null,
      map: null,
      activeRegion: null,
      installedRegions: [],
      pendingClick: null,
      pendingContextmenu: null,
      leftRailTab: "search",
      poiVisibility: defaultPoiVisibility(),
      activeCategoryChip: null,
      categorySearchResults: [],
      categorySearchBbox: null,
    }),
}));
