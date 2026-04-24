"use client";

import { create } from "zustand";
import type { Map as MapLibreMap } from "maplibre-gl";
import type { MapViewport } from "@/lib/url-state";
import type { GeocodeResult, Region } from "@/lib/api/schemas";

/**
 * Client-only map state. Server state (routes, POIs, geocode results,
 * region list) is owned by TanStack Query — this store holds only what
 * the user's latest interaction produced in the browser plus the shared
 * references other feature modules need (MapLibre instance, pending
 * click/contextmenu, the active region key).
 */

/** Selected POI (from `/api/pois/{id}` or a click-resolve). */
export interface SelectedPoi {
  id: string;
  label: string;
  lat: number;
  lon: number;
  /** Free-form subtitle, e.g. address or category. */
  subtitle?: string;
}

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
  | "results"
  | "directions"
  | "place"
  | "saved";

export interface MapState {
  viewport: MapViewport;
  selectedPoi: SelectedPoi | null;
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

  /** The GeocodeResult the user most recently picked from search. */
  selectedResult: GeocodeResult | null;
  /** Which tab of the left rail is currently visible. */
  leftRailTab: LeftRailTab;

  setViewport: (viewport: MapViewport) => void;
  setSelectedPoi: (poi: SelectedPoi | null) => void;
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
  setSelectedResult: (result: GeocodeResult | null) => void;
  /** Switch the left rail to the given tab (and ensure it's rendered). */
  openLeftRail: (tab: LeftRailTab) => void;
  /** Resets every field to its initial state; used on sign-out / tests. */
  clear: () => void;
}

/** Initial viewport when URL hash is absent and settings provide no default. */
export const DEFAULT_VIEWPORT: MapViewport = {
  lat: 0,
  lon: 0,
  zoom: 2,
  bearing: 0,
  pitch: 0,
};

export const useMapStore = create<MapState>((set) => ({
  viewport: DEFAULT_VIEWPORT,
  selectedPoi: null,
  activeRoute: null,

  map: null,
  activeRegion: null,
  installedRegions: [],

  pendingClick: null,
  pendingContextmenu: null,

  selectedResult: null,
  leftRailTab: "search",

  setViewport: (viewport) => set({ viewport }),
  setSelectedPoi: (selectedPoi) => set({ selectedPoi }),
  setActiveRoute: (activeRoute) => set({ activeRoute }),
  setMap: (map) => set({ map }),
  setActiveRegion: (activeRegion) => set({ activeRegion }),
  setInstalledRegions: (installedRegions) => set({ installedRegions }),
  setPendingClick: (pendingClick) => set({ pendingClick }),
  setPendingContextmenu: (pendingContextmenu) => set({ pendingContextmenu }),
  clearPendingClick: () => set({ pendingClick: null }),
  clearPendingContextmenu: () => set({ pendingContextmenu: null }),
  setSelectedResult: (selectedResult) => set({ selectedResult }),
  openLeftRail: (leftRailTab) => set({ leftRailTab }),
  clear: () =>
    set({
      viewport: DEFAULT_VIEWPORT,
      selectedPoi: null,
      activeRoute: null,
      map: null,
      activeRegion: null,
      installedRegions: [],
      pendingClick: null,
      pendingContextmenu: null,
      selectedResult: null,
      leftRailTab: "search",
    }),
}));
