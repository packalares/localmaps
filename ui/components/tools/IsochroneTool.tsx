"use client";

import { useEffect } from "react";
import { registerLayer, unregisterLayer } from "@/lib/map/layer-bus";
import { useMapStore } from "@/lib/state/map";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import { useIsochroneStore } from "@/lib/tools/isochrone-state";

const ISO_FILL_ID = "isochrone-fill";
const ISO_LINE_ID = "isochrone-line";

/**
 * Non-visual driver for the isochrone tool. Two responsibilities:
 *
 * 1. While active + origin missing, capture the next map click into the
 *    isochrone store as the origin. After that the UI panel stays in
 *    charge until the user clicks "Render".
 *
 * 2. Whenever a `result` FeatureCollection arrives, register it on the
 *    map as a fill layer (with graduated opacity by contour seconds) +
 *    a matching outline. Previous layers are always replaced so
 *    re-rendering is idempotent.
 */
export function IsochroneTool() {
  const active = useActiveToolStore((s) => s.active);
  const map = useMapStore((s) => s.map);
  const pendingClick = useMapStore((s) => s.pendingClick);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);

  const origin = useIsochroneStore((s) => s.origin);
  const setOrigin = useIsochroneStore((s) => s.setOrigin);
  const result = useIsochroneStore((s) => s.result);

  // Capture origin click while the tool is live and no origin is set yet.
  useEffect(() => {
    if (active !== "isochrone") return;
    if (origin) return;
    if (!pendingClick) return;
    setOrigin({
      lng: pendingClick.lngLat.lng,
      lat: pendingClick.lngLat.lat,
    });
    clearPendingClick();
  }, [active, origin, pendingClick, setOrigin, clearPendingClick]);

  // Paint the result polygons when they arrive.
  useEffect(() => {
    if (!map) return;
    if (active !== "isochrone" || !result || result.features.length === 0) {
      unregisterLayer(map, ISO_FILL_ID);
      unregisterLayer(map, ISO_LINE_ID);
      return;
    }

    // Sort so larger contours render underneath smaller ones. The
    // gateway's GeoJSON features may carry `contour` or `time` props;
    // we fall back to feature order if neither is present. The schema
    // validates only the envelope; coordinates stay unknown so we hand
    // the payload to MapLibre as an opaque GeoJSON source (cast is safe
    // because MapLibre does its own runtime parsing).
    const features = [...result.features].sort((a, b) => contourKey(b) - contourKey(a));
    const fc = { type: "FeatureCollection" as const, features } as unknown as GeoJSON.FeatureCollection;

    registerLayer(
      map,
      ISO_FILL_ID,
      { type: "geojson", data: fc },
      {
        type: "fill",
        paint: {
          "fill-color": [
            "interpolate",
            ["linear"],
            ["coalesce", ["get", "contour"], ["get", "time"], 0],
            0,
            "#16a34a",
            900,
            "#facc15",
            1800,
            "#f97316",
          ] as unknown as string,
          "fill-opacity": 0.28,
        },
      },
    );
    registerLayer(
      map,
      ISO_LINE_ID,
      { type: "geojson", data: fc },
      {
        type: "line",
        paint: { "line-color": "#1d4ed8", "line-width": 1 },
      },
    );
  }, [map, active, result]);

  // Cleanup layers on unmount.
  useEffect(() => {
    return () => {
      const m = useMapStore.getState().map;
      if (!m) return;
      unregisterLayer(m, ISO_FILL_ID);
      unregisterLayer(m, ISO_LINE_ID);
    };
  }, []);

  return null;
}

/**
 * Return a numeric contour key for sorting. Valhalla emits
 * `properties.contour` in minutes; openrouteservice uses `time` in
 * seconds. We accept both and coerce the minute value to seconds.
 */
function contourKey(f: { properties?: Record<string, unknown> | null }): number {
  const p = f.properties ?? {};
  if (typeof p.contour === "number") return p.contour * 60;
  if (typeof p.time === "number") return p.time;
  return 0;
}
