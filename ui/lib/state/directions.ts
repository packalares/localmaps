"use client";

import { create } from "zustand";
import type { Route, RouteMode } from "@/lib/api/schemas";
import { reorder } from "@/lib/directions/waypoint-reorder";

/**
 * Client-only slice owned by the Directions panel. Actual route data
 * comes from TanStack Query (`useRoute` mutation) — this store holds
 * the in-flight inputs plus the last-rendered route so other parts of
 * the UI (map polyline, export menu, context menu) can read them.
 */

export interface Waypoint {
  /** Stable UUID so TanStack Query cache keys don't churn on re-ordering. */
  id: string;
  /** Human-readable label; always displayable even when placeholder. */
  label: string;
  /** Resolved coordinate. null while the user is still picking. */
  lngLat: { lng: number; lat: number } | null;
  /** Free-form placeholder (e.g. "Choose starting point"). */
  placeholder?: string;
}

export interface RouteOptions {
  avoidHighways: boolean;
  avoidTolls: boolean;
  avoidFerries: boolean;
}

export interface DirectionsState {
  waypoints: Waypoint[];
  mode: RouteMode;
  options: RouteOptions;
  /** Route selected for rendering (one of the `routes[]` from /api/route). */
  route: Route | null;
  /** All routes returned by the most recent /api/route call. */
  alternatives: Route[];

  setMode: (mode: RouteMode) => void;
  setOptions: (options: Partial<RouteOptions>) => void;
  setWaypoint: (index: number, patch: Partial<Waypoint>) => void;
  setWaypointFromPoint: (
    index: number,
    lngLat: { lng: number; lat: number },
    label?: string,
  ) => void;
  /** Convenience for the "Directions from here" context-menu action. */
  setStartFromPoint: (
    lngLat: { lng: number; lat: number },
    label?: string,
  ) => void;
  /** Convenience for the "Directions to here" context-menu action. */
  setEndFromPoint: (
    lngLat: { lng: number; lat: number },
    label?: string,
  ) => void;
  addWaypoint: () => void;
  removeWaypoint: (index: number) => void;
  reorderWaypoints: (from: number, to: number) => void;
  swapEnds: () => void;
  setRoute: (route: Route | null, alternatives?: Route[]) => void;
  reset: () => void;
}

function uuid(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  // Fallback for older test envs.
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}

function emptyWaypoint(placeholder: string): Waypoint {
  return { id: uuid(), label: "", lngLat: null, placeholder };
}

export const DEFAULT_WAYPOINTS = (): Waypoint[] => [
  emptyWaypoint("Choose starting point"),
  emptyWaypoint("Choose destination"),
];

export const DEFAULT_OPTIONS: RouteOptions = {
  avoidHighways: false,
  avoidTolls: false,
  avoidFerries: false,
};

export const useDirectionsStore = create<DirectionsState>((set, get) => ({
  waypoints: DEFAULT_WAYPOINTS(),
  mode: "auto",
  options: DEFAULT_OPTIONS,
  route: null,
  alternatives: [],

  setMode: (mode) => set({ mode }),
  setOptions: (patch) =>
    set((state) => ({ options: { ...state.options, ...patch } })),

  setWaypoint: (index, patch) =>
    set((state) => {
      if (index < 0 || index >= state.waypoints.length) return state;
      const next = [...state.waypoints];
      next[index] = { ...next[index], ...patch };
      return { waypoints: next };
    }),

  setWaypointFromPoint: (index, lngLat, label) => {
    const state = get();
    if (index < 0 || index >= state.waypoints.length) return;
    const next = [...state.waypoints];
    next[index] = {
      ...next[index],
      lngLat,
      label: label ?? "Dropped pin",
    };
    set({ waypoints: next });
  },

  setStartFromPoint: (lngLat, label) => {
    get().setWaypointFromPoint(0, lngLat, label ?? "Dropped pin");
  },

  setEndFromPoint: (lngLat, label) => {
    const state = get();
    const lastIndex = state.waypoints.length - 1;
    state.setWaypointFromPoint(
      Math.max(1, lastIndex),
      lngLat,
      label ?? "Dropped pin",
    );
  },

  addWaypoint: () =>
    set((state) => {
      const next = [...state.waypoints];
      const insertAt = Math.max(1, next.length - 1);
      next.splice(insertAt, 0, emptyWaypoint("Add stop"));
      return { waypoints: next };
    }),

  removeWaypoint: (index) =>
    set((state) => {
      if (state.waypoints.length <= 2) return state;
      const next = state.waypoints.filter((_, i) => i !== index);
      return { waypoints: next };
    }),

  reorderWaypoints: (from, to) =>
    set((state) => ({
      waypoints: reorder(state.waypoints, from, to),
    })),

  swapEnds: () =>
    set((state) => {
      const n = state.waypoints.length;
      if (n < 2) return state;
      const next = [...state.waypoints];
      const a = next[0];
      const b = next[n - 1];
      next[0] = b;
      next[n - 1] = a;
      return { waypoints: next };
    }),

  setRoute: (route, alternatives) =>
    set({ route, alternatives: alternatives ?? (route ? [route] : []) }),

  reset: () =>
    set({
      waypoints: DEFAULT_WAYPOINTS(),
      mode: "auto",
      options: DEFAULT_OPTIONS,
      route: null,
      alternatives: [],
    }),
}));
