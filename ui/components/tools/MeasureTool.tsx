"use client";

import { useEffect } from "react";
import { registerLayer, unregisterLayer } from "@/lib/map/layer-bus";
import { useMapStore } from "@/lib/state/map";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import { useMeasureStore } from "@/lib/tools/measure-state";

const LINE_LAYER_ID = "measure-line";
const FILL_LAYER_ID = "measure-fill";
const POINTS_LAYER_ID = "measure-points";

/**
 * Non-visual component that owns the measure interaction. Consumes
 * `pendingClick` from the map store to append points, and registers
 * three layers via the layer-bus:
 *
 * - fill (area mode only, once ≥3 points)
 * - the live polyline
 * - circle markers at every point
 *
 * Keyboard controls live here so they're tied to the tool's lifetime
 * rather than any particular visual component.
 */
export function MeasureTool() {
  const active = useActiveToolStore((s) => s.active);
  const closeAll = useActiveToolStore((s) => s.closeAll);

  const map = useMapStore((s) => s.map);
  const pendingClick = useMapStore((s) => s.pendingClick);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);

  const mode = useMeasureStore((s) => s.mode);
  const points = useMeasureStore((s) => s.points);
  const isActive = useMeasureStore((s) => s.isActive);
  const isFinalised = useMeasureStore((s) => s.isFinalised);
  const addPoint = useMeasureStore((s) => s.addPoint);
  const removeLastPoint = useMeasureStore((s) => s.removeLastPoint);
  const finalise = useMeasureStore((s) => s.finalise);

  // --- Map-click → addPoint ------------------------------------------
  useEffect(() => {
    if (active !== "measure" || !isActive || isFinalised) return;
    if (!pendingClick) return;
    addPoint({
      lng: pendingClick.lngLat.lng,
      lat: pendingClick.lngLat.lat,
    });
    clearPendingClick();
  }, [active, isActive, isFinalised, pendingClick, addPoint, clearPendingClick]);

  // --- Keyboard ------------------------------------------------------
  useEffect(() => {
    if (active !== "measure") return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        closeAll();
      } else if (e.key === "Backspace") {
        // Only intercept when the focus target is not an input (so we
        // don't steal Backspace from typed search boxes).
        const t = e.target as HTMLElement | null;
        const tag = t?.tagName?.toLowerCase();
        if (tag === "input" || tag === "textarea") return;
        e.preventDefault();
        removeLastPoint();
      } else if (e.key === "Enter") {
        e.preventDefault();
        finalise();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, closeAll, removeLastPoint, finalise]);

  // --- Double-click finalises ---------------------------------------
  useEffect(() => {
    if (active !== "measure" || !map) return;
    const onDblClick = () => finalise();
    map.on("dblclick", onDblClick);
    return () => {
      map.off("dblclick", onDblClick);
    };
  }, [active, map, finalise]);

  // --- Layer rendering ----------------------------------------------
  useEffect(() => {
    if (!map) return;
    if (active !== "measure" || points.length === 0) {
      unregisterLayer(map, POINTS_LAYER_ID);
      unregisterLayer(map, LINE_LAYER_ID);
      unregisterLayer(map, FILL_LAYER_ID);
      return;
    }

    const coords = points.map((p) => [p.lng, p.lat]);
    const lineFc = {
      type: "FeatureCollection" as const,
      features:
        points.length >= 2
          ? [
              {
                type: "Feature" as const,
                geometry: {
                  type: "LineString" as const,
                  coordinates:
                    mode === "area" && points.length >= 3
                      ? [...coords, coords[0]!]
                      : coords,
                },
                properties: {},
              },
            ]
          : [],
    };
    registerLayer(
      map,
      LINE_LAYER_ID,
      { type: "geojson", data: lineFc },
      {
        type: "line",
        layout: { "line-cap": "round", "line-join": "round" },
        paint: {
          "line-color": "#2563eb",
          "line-width": 3,
          "line-dasharray": isFinalised ? [1, 0] : [2, 2],
        },
      },
    );

    if (mode === "area" && points.length >= 3) {
      const fillFc = {
        type: "FeatureCollection" as const,
        features: [
          {
            type: "Feature" as const,
            geometry: {
              type: "Polygon" as const,
              coordinates: [[...coords, coords[0]!]],
            },
            properties: {},
          },
        ],
      };
      registerLayer(
        map,
        FILL_LAYER_ID,
        { type: "geojson", data: fillFc },
        {
          type: "fill",
          paint: { "fill-color": "#2563eb", "fill-opacity": 0.12 },
        },
      );
    } else {
      unregisterLayer(map, FILL_LAYER_ID);
    }

    registerLayer(
      map,
      POINTS_LAYER_ID,
      {
        type: "geojson",
        data: {
          type: "FeatureCollection",
          features: points.map((p) => ({
            type: "Feature" as const,
            geometry: { type: "Point" as const, coordinates: [p.lng, p.lat] },
            properties: {},
          })),
        },
      },
      {
        type: "circle",
        paint: {
          "circle-radius": 5,
          "circle-color": "#ffffff",
          "circle-stroke-color": "#2563eb",
          "circle-stroke-width": 2,
        },
      },
    );
  }, [map, active, mode, points, isFinalised]);

  // Teardown on deactivate / unmount.
  useEffect(() => {
    return () => {
      const m = useMapStore.getState().map;
      if (!m) return;
      unregisterLayer(m, POINTS_LAYER_ID);
      unregisterLayer(m, LINE_LAYER_ID);
      unregisterLayer(m, FILL_LAYER_ID);
    };
  }, []);

  return null;
}
