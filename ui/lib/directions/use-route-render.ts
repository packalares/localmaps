"use client";

import { useEffect } from "react";
import { registerLayer, unregisterLayer } from "@/lib/map/layer-bus";
import { useMapStore } from "@/lib/state/map";
import { useDirectionsStore } from "@/lib/state/directions";
import { boundsFromPoints, decodePolyline, type LngLat } from "./polyline";

export const LAYER_ID = "directions-route";
export const UNDERLAY_ID = "directions-route-underlay";
export const MARKERS_ID = "directions-waypoints";

/**
 * Wires the currently-selected route into the MapLibre instance:
 * paints the polyline (with white underlay), puts coloured markers on
 * the waypoints, fits the viewport to the route bounds, and mirrors
 * the summary into the shared `activeRoute` store entry.
 *
 * Returns nothing — all side effects go through the `layer-bus` +
 * `useMapStore`. Cleanup runs on unmount so the map never keeps a
 * dangling source/layer after the panel closes.
 */
export function useRouteRender(): void {
  const map = useMapStore((s) => s.map);
  const setActiveRoute = useMapStore((s) => s.setActiveRoute);
  const route = useDirectionsStore((s) => s.route);
  const waypoints = useDirectionsStore((s) => s.waypoints);

  useEffect(() => {
    if (!map) return;
    if (!route) {
      setActiveRoute(null);
      unregisterLayer(map, LAYER_ID);
      unregisterLayer(map, UNDERLAY_ID);
      unregisterLayer(map, MARKERS_ID);
      return;
    }
    const combined: LngLat[] = route.legs.flatMap((l) =>
      decodePolyline(l.geometry.polyline, 6),
    );
    if (combined.length === 0) return;

    const lineGeoJson = {
      type: "FeatureCollection" as const,
      features: [
        {
          type: "Feature" as const,
          geometry: {
            type: "LineString" as const,
            coordinates: combined.map((p) => [p.lng, p.lat]),
          },
          properties: {},
        },
      ],
    };
    registerLayer(
      map,
      UNDERLAY_ID,
      { type: "geojson", data: lineGeoJson },
      {
        type: "line",
        layout: { "line-cap": "round", "line-join": "round" },
        paint: { "line-color": "#ffffff", "line-width": 8 },
      },
    );
    registerLayer(
      map,
      LAYER_ID,
      { type: "geojson", data: lineGeoJson },
      {
        type: "line",
        layout: { "line-cap": "round", "line-join": "round" },
        paint: { "line-color": "#2b85ec", "line-width": 6 },
      },
    );

    const waypointFeatures = waypoints
      .map((w, i) => {
        if (!w.lngLat) return null;
        const kind =
          i === 0 ? "start" : i === waypoints.length - 1 ? "end" : "via";
        return {
          type: "Feature" as const,
          geometry: {
            type: "Point" as const,
            coordinates: [w.lngLat.lng, w.lngLat.lat],
          },
          properties: { kind },
        };
      })
      .filter((f): f is NonNullable<typeof f> => f !== null);
    registerLayer(
      map,
      MARKERS_ID,
      {
        type: "geojson",
        data: { type: "FeatureCollection", features: waypointFeatures },
      },
      {
        type: "circle",
        paint: {
          "circle-radius": 7,
          "circle-stroke-color": "#ffffff",
          "circle-stroke-width": 2,
          "circle-color": [
            "match",
            ["get", "kind"],
            "start",
            "#22c55e",
            "end",
            "#ef4444",
            "#6b7280",
          ],
        },
      },
    );

    const bounds = boundsFromPoints(combined);
    if (bounds) {
      try {
        map.fitBounds(
          [
            [bounds.bbox[0], bounds.bbox[1]],
            [bounds.bbox[2], bounds.bbox[3]],
          ],
          {
            padding: { top: 80, right: 80, bottom: 80, left: 380 },
            duration: 400,
          },
        );
      } catch {
        // Map may not be fully ready when a route is set via deep-link.
      }
    }

    setActiveRoute({
      id: route.id,
      mode: route.mode ?? "auto",
      polyline: route.legs.map((l) => l.geometry.polyline).join(""),
      distanceMeters: route.summary?.distanceMeters ?? 0,
      timeSeconds: route.summary?.timeSeconds ?? 0,
    });
  }, [map, route, waypoints, setActiveRoute]);

  useEffect(() => {
    return () => {
      const m = useMapStore.getState().map;
      if (!m) return;
      unregisterLayer(m, LAYER_ID);
      unregisterLayer(m, UNDERLAY_ID);
      unregisterLayer(m, MARKERS_ID);
      useMapStore.getState().setActiveRoute(null);
    };
  }, []);
}
