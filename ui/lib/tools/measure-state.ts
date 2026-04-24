"use client";

import { create } from "zustand";
import type { LngLatPt } from "./geometry";

/**
 * Zustand slice driving the measure tool. Only one measurement is live
 * at a time; the FAB popover's "Close all" action calls `clear`.
 *
 * The tool has two flavours: a polyline distance sum and a polygon
 * area. The points array is shared; the UI reads it once per render
 * to paint the live line and the summary panel.
 */
export type MeasureMode = "distance" | "area";

export interface MeasureState {
  mode: MeasureMode;
  points: LngLatPt[];
  /** True while the tool is capturing map clicks. */
  isActive: boolean;
  /** Once finalised the line stays painted but new clicks no longer add. */
  isFinalised: boolean;

  setMode: (mode: MeasureMode) => void;
  setActive: (active: boolean) => void;
  addPoint: (pt: LngLatPt) => void;
  removeLastPoint: () => void;
  finalise: () => void;
  clear: () => void;
}

export const useMeasureStore = create<MeasureState>((set) => ({
  mode: "distance",
  points: [],
  isActive: false,
  isFinalised: false,

  setMode: (mode) => set({ mode }),
  setActive: (active) =>
    set((s) => ({
      isActive: active,
      // Starting a new measurement resets previous state so the live
      // paint doesn't accumulate across sessions.
      points: active ? [] : s.points,
      isFinalised: active ? false : s.isFinalised,
    })),
  addPoint: (pt) =>
    set((s) => {
      if (!s.isActive || s.isFinalised) return s;
      return { points: [...s.points, pt] };
    }),
  removeLastPoint: () =>
    set((s) => {
      if (!s.isActive || s.isFinalised || s.points.length === 0) return s;
      return { points: s.points.slice(0, -1) };
    }),
  finalise: () =>
    set((s) => ({
      isFinalised: s.points.length > 0,
      isActive: false,
    })),
  clear: () =>
    set({ mode: "distance", points: [], isActive: false, isFinalised: false }),
}));
