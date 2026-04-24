"use client";

import { useMemo } from "react";
import type { Poi } from "@/lib/api/schemas";
import { useNearestPoi, usePoi } from "@/lib/api/hooks";

/**
 * Resolution of a map click into a POI. Callers pass either:
 *
 *   - `poiId`   (click landed on a tiled feature with a poi_id), or
 *   - `lngLat`  (click in empty space → find the nearest POI)
 *
 * The hook fires the appropriate query and returns the canonical
 * `{poi, status}` pair the `PoiPane` component understands.
 *
 * Public API so `ContextMenu` (Agent K) + primary's merge can call it
 * without knowing about our internal query keys.
 */
export interface WhatsHereHit {
  poiId?: string | null;
  lngLat?: { lng: number; lat: number } | null;
  /** Default 50 m; passed through to `useNearestPoi`. */
  radius?: number;
}

export interface UseWhatsHereResult {
  poi: Poi | null;
  status: "idle" | "loading" | "error";
}

export function useWhatsHere(hit: WhatsHereHit | null): UseWhatsHereResult {
  const byId = usePoi(hit?.poiId ?? null, {
    enabled: !!hit?.poiId,
  });
  const byPoint = useNearestPoi({
    lngLat: hit?.poiId ? null : hit?.lngLat ?? null,
    radius: hit?.radius ?? 50,
    enabled: !hit?.poiId && !!hit?.lngLat,
  });

  return useMemo<UseWhatsHereResult>(() => {
    if (!hit) return { poi: null, status: "idle" };
    if (hit.poiId) {
      if (byId.isLoading) return { poi: null, status: "loading" };
      if (byId.isError) return { poi: null, status: "error" };
      return { poi: byId.data ?? null, status: "idle" };
    }
    if (byPoint.isLoading) return { poi: null, status: "loading" };
    if (byPoint.isError) return { poi: null, status: "error" };
    return { poi: byPoint.data ?? null, status: "idle" };
  }, [
    hit,
    byId.isLoading,
    byId.isError,
    byId.data,
    byPoint.isLoading,
    byPoint.isError,
    byPoint.data,
  ]);
}
