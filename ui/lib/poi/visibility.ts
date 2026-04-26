import type { Map as MapLibreMap } from "maplibre-gl";
import { POI_CATEGORIES, type PoiCategory } from "@/lib/state/map";

/**
 * Runtime sync between the Zustand `poiVisibility` map and the live
 * MapLibre style. The style contract (agreed with the style-rewrite
 * agent) ships paired layers for each category:
 *
 *   poi-<category>        — icon layer
 *   poi-<category>-label  — label layer (optional, not every style ships)
 *
 * We flip both in lockstep so a hidden category disappears completely
 * rather than leaving dangling text.
 */

/** All layer ids touched for a given category, in the order we apply them. */
export function layerIdsFor(cat: PoiCategory): readonly string[] {
  return [`poi-${cat}`, `poi-${cat}-label`];
}

/** Minimal MapLibre surface we need; kept narrow for test doubles. */
export interface VisibilityMap {
  getLayer: (id: string) => unknown;
  setLayoutProperty: (
    layerId: string,
    name: string,
    value: unknown,
  ) => void;
}

/**
 * Apply a single (category, visible) toggle to every matching layer on
 * the map. Layers that don't exist yet (style still loading, this style
 * lacks labels, etc.) are skipped silently — the caller can re-apply
 * after the style finishes loading.
 *
 * Returns the number of layers actually mutated, which tests use to
 * assert that the sync found the layers it expected.
 */
export function applyCategoryVisibility(
  map: VisibilityMap | MapLibreMap | null | undefined,
  cat: PoiCategory,
  visible: boolean,
): number {
  if (!map) return 0;
  const target = visible ? "visible" : "none";
  let touched = 0;
  for (const id of layerIdsFor(cat)) {
    try {
      if (!map.getLayer(id)) continue;
      map.setLayoutProperty(id, "visibility", target);
      touched += 1;
    } catch {
      // MapLibre throws if the layer was removed between `getLayer` and
      // `setLayoutProperty`. Treat as benign.
    }
  }
  return touched;
}

/**
 * Push the full `poiVisibility` map onto the live MapLibre style. Call
 * this on style load and on every visibility change; it's idempotent and
 * cheap — 9 categories × 2 layers × a single layout write each.
 */
export function applyAllPoiVisibility(
  map: VisibilityMap | MapLibreMap | null | undefined,
  visibility: Record<PoiCategory, boolean>,
): number {
  if (!map) return 0;
  let touched = 0;
  for (const c of POI_CATEGORIES) {
    touched += applyCategoryVisibility(map, c, visibility[c] ?? true);
  }
  return touched;
}
