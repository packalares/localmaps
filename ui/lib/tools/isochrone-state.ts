"use client";

import { create } from "zustand";
import type { IsochroneResponse, RouteMode } from "@/lib/api/schemas";
import type { LngLatPt } from "./geometry";

/**
 * Zustand slice driving the isochrone tool. Phase 7 surface mirrors the
 * Valhalla/OpenRouteService flow the gateway implements:
 *
 *   origin  → user-clicked coordinate (or falls back to map centre)
 *   mode    → `auto | bicycle | pedestrian` (RouteMode, per openapi)
 *   minutes → contour band(s) in minutes; serialised to `contoursSeconds`
 *
 * The `result` field holds the last successful GeoJSON response so the
 * isochrone renderer can re-register layers without re-firing the API.
 */

export type IsochroneMode = Extract<RouteMode, "auto" | "bicycle" | "pedestrian">;

/** Minute bands offered in the UI. The contract is `contoursSeconds`
 * (integer seconds), so selecting `15` yields `900` on the wire. */
export const AVAILABLE_MINUTES = [10, 15, 30] as const;
export type AvailableMinutes = (typeof AVAILABLE_MINUTES)[number];

export interface IsochroneSliceState {
  origin: LngLatPt | null;
  mode: IsochroneMode;
  minutes: AvailableMinutes[];
  result: IsochroneResponse | null;
  isLoading: boolean;
  /** True while the tool is capturing the origin click. */
  isActive: boolean;

  setActive: (active: boolean) => void;
  setOrigin: (origin: LngLatPt | null) => void;
  setMode: (mode: IsochroneMode) => void;
  toggleMinutes: (m: AvailableMinutes) => void;
  setResult: (result: IsochroneResponse | null) => void;
  setLoading: (loading: boolean) => void;
  clear: () => void;
}

export const useIsochroneStore = create<IsochroneSliceState>((set) => ({
  origin: null,
  mode: "auto",
  minutes: [10, 15, 30],
  result: null,
  isLoading: false,
  isActive: false,

  setActive: (active) =>
    set((s) => ({
      isActive: active,
      // When activating, clear any prior result so the stale polygon
      // does not linger while the user picks a new origin.
      result: active ? null : s.result,
      origin: active ? null : s.origin,
    })),
  setOrigin: (origin) => set({ origin }),
  setMode: (mode) => set({ mode }),
  toggleMinutes: (m) =>
    set((s) => {
      const has = s.minutes.includes(m);
      const next = has
        ? s.minutes.filter((x) => x !== m)
        : [...s.minutes, m].sort((a, b) => a - b);
      // Never leave the UI with zero bands — keep at least one
      // checked so the render button has something to ask for.
      if (next.length === 0) return s;
      return { minutes: next };
    }),
  setResult: (result) => set({ result }),
  setLoading: (loading) => set({ isLoading: loading }),
  clear: () =>
    set({
      origin: null,
      mode: "auto",
      minutes: [10, 15, 30],
      result: null,
      isLoading: false,
      isActive: false,
    }),
}));
