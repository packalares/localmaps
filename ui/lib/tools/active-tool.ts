"use client";

import { create } from "zustand";
import { useIsochroneStore } from "./isochrone-state";
import { useMeasureStore } from "./measure-state";

/**
 * Primary switch driving "only one tool at a time". When a tool is set
 * active we call its start-of-session reset on the owning slice and
 * tear the other slice down. The FAB popover, the ToolsFab and the
 * individual tool components all read/write this switch.
 */
export type ActiveTool = "measure" | "isochrone" | null;

export interface ActiveToolState {
  active: ActiveTool;
  setActive: (tool: ActiveTool) => void;
  /** Close the current tool without starting a new one. */
  closeAll: () => void;
}

export const useActiveToolStore = create<ActiveToolState>((set) => ({
  active: null,
  setActive: (tool) => {
    // Owner slices are reset via their own `clear`/`setActive(false)` so
    // each tool keeps its own teardown logic co-located with the slice.
    if (tool !== "measure") useMeasureStore.getState().clear();
    if (tool !== "isochrone") useIsochroneStore.getState().clear();
    if (tool === "measure") useMeasureStore.getState().setActive(true);
    if (tool === "isochrone") useIsochroneStore.getState().setActive(true);
    set({ active: tool });
  },
  closeAll: () => {
    useMeasureStore.getState().clear();
    useIsochroneStore.getState().clear();
    set({ active: null });
  },
}));
