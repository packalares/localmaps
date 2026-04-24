"use client";

import { useLayoutEffect, useRef } from "react";
import { useMapStore } from "@/lib/state/map";
import {
  useDirectionsStore,
  type Waypoint,
} from "@/lib/state/directions";
import { decodeURL } from "./decode";
import type { DecodedState } from "./types";

/**
 * On-mount restorer. Reads `window.location` once, decodes it, and
 * pushes the resulting slice values into the Zustand stores. Runs in a
 * `useLayoutEffect` so the first `MapView` render sees the restored
 * viewport — the viewport is also owned by `useUrlViewport`, which is
 * what MapView already consumes, so the net effect is belt-and-braces:
 * this hook fills the non-viewport fields (tab / poi / route / query).
 *
 * No-op on SSR.
 */
export function useRestoreOnMount(): void {
  const hasRun = useRef(false);
  useLayoutEffect(() => {
    if (hasRun.current) return;
    if (typeof window === "undefined") return;
    hasRun.current = true;

    const decoded = decodeURL(window.location.href);
    if (!decoded) return;
    applyDecoded(decoded);
  }, []);
}

/** Apply a `DecodedState` to the two client stores. Exported for tests. */
export function applyDecoded(decoded: DecodedState): void {
  const mapStore = useMapStore.getState();
  const dirStore = useDirectionsStore.getState();

  if (decoded.viewport) {
    mapStore.setViewport(decoded.viewport);
  }
  if (decoded.activeRegion !== undefined) {
    mapStore.setActiveRegion(decoded.activeRegion);
  }
  if (decoded.leftRailTab) {
    mapStore.openLeftRail(decoded.leftRailTab);
  }
  if (decoded.selectedPoiId) {
    // We only have an id from the URL — stamp a minimal shell so the UI
    // can immediately reflect "a POI is selected"; a full fetch via
    // `/api/pois/{id}` is outside this agent's scope.
    mapStore.setSelectedPoi({
      id: decoded.selectedPoiId,
      label: "",
      lat: decoded.viewport?.lat ?? 0,
      lon: decoded.viewport?.lon ?? 0,
    });
  }

  if (decoded.route) {
    dirStore.setMode(decoded.route.mode);
    dirStore.setOptions({
      avoidHighways: decoded.route.options.avoidHighways,
      avoidTolls: decoded.route.options.avoidTolls,
      avoidFerries: decoded.route.options.avoidFerries,
    });

    // Re-materialise waypoints. Keep stable ids so consumers don't churn.
    const incoming = decoded.route.waypoints;
    const existing = dirStore.waypoints;
    const next: Waypoint[] = incoming.map((wp, i) => ({
      id: existing[i]?.id ?? uuid(),
      label: existing[i]?.label ?? "Shared pin",
      lngLat: { lng: wp.lng, lat: wp.lat },
      placeholder:
        i === 0
          ? "Choose starting point"
          : i === incoming.length - 1
          ? "Choose destination"
          : "Add stop",
    }));
    // Ensure at least 2 waypoints so the directions UI renders correctly.
    while (next.length < 2) {
      next.push({
        id: uuid(),
        label: "",
        lngLat: null,
        placeholder:
          next.length === 0 ? "Choose starting point" : "Choose destination",
      });
    }
    useDirectionsStore.setState({ waypoints: next });
  }
}

function uuid(): string {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return Math.random().toString(36).slice(2) + Date.now().toString(36);
}
