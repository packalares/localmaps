import type { Map as MapLibreMap, GeoJSONSource } from "maplibre-gl";
import type { Poi } from "@/lib/api/schemas";

/**
 * Pulsing-dot highlight for the currently-selected POI. We keep the
 * MapLibre manipulation behind a tiny ambient interface so tests (and
 * future callers from Agent I's layer-bus) can drive it with a stub.
 *
 * The layer id is namespaced to avoid colliding with I/J/K layers:
 *   source id: "localmaps-poi-highlight"
 *   layer id:  "localmaps-poi-highlight-dot"
 *
 * TODO: switch to I's `ui/lib/map/layer-bus.ts` registry when it
 * lands; today we talk to MapLibre directly.
 */

export const HIGHLIGHT_SOURCE_ID = "localmaps-poi-highlight";
export const HIGHLIGHT_LAYER_ID = "localmaps-poi-highlight-dot";

/**
 * Minimal MapLibre surface we touch. Kept as a structural type so tests
 * don't need a full `maplibre-gl` Map instance.
 */
export interface HighlightMap {
  getSource(id: string): unknown;
  getLayer(id: string): unknown;
  addSource(id: string, source: unknown): void;
  addLayer(layer: unknown): void;
  removeLayer(id: string): void;
  removeSource(id: string): void;
}

function featureFor(poi: Pick<Poi, "center">) {
  return {
    type: "FeatureCollection" as const,
    features: [
      {
        type: "Feature" as const,
        geometry: {
          type: "Point" as const,
          coordinates: [poi.center.lon, poi.center.lat],
        },
        properties: {},
      },
    ],
  };
}

/**
 * Ensures a pulsing-dot layer is present on the map for the given POI.
 * Safe to call repeatedly; updates the existing source in place rather
 * than tearing the layer down. Returns true if a change was applied.
 */
export function setPoiHighlight(
  map: HighlightMap | null | undefined,
  poi: Pick<Poi, "center"> | null,
): boolean {
  if (!map) return false;

  if (!poi) {
    return clearPoiHighlight(map);
  }

  const data = featureFor(poi);
  const existing = map.getSource(HIGHLIGHT_SOURCE_ID) as
    | (GeoJSONSource & { setData: (d: unknown) => void })
    | undefined;

  if (existing && typeof existing.setData === "function") {
    existing.setData(data);
    return true;
  }

  map.addSource(HIGHLIGHT_SOURCE_ID, {
    type: "geojson",
    data,
  });

  if (!map.getLayer(HIGHLIGHT_LAYER_ID)) {
    map.addLayer({
      id: HIGHLIGHT_LAYER_ID,
      source: HIGHLIGHT_SOURCE_ID,
      type: "circle",
      paint: {
        "circle-radius": [
          "interpolate",
          ["linear"],
          ["zoom"],
          10,
          6,
          16,
          14,
        ],
        "circle-color": "#1a73e8",
        "circle-opacity": 0.9,
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 3,
        "circle-stroke-opacity": 0.95,
        // A simple pulse via MapLibre's time-driven opacity isn't in the
        // style spec; true pulsing is done with a DOM marker but the
        // circle here is a static fallback that renders regardless of
        // browser. Agent I may upgrade to a marker when layer-bus lands.
      },
    });
  }
  return true;
}

/** Removes the highlight layer + source if present. */
export function clearPoiHighlight(map: HighlightMap | null | undefined): boolean {
  if (!map) return false;
  let changed = false;
  if (map.getLayer(HIGHLIGHT_LAYER_ID)) {
    map.removeLayer(HIGHLIGHT_LAYER_ID);
    changed = true;
  }
  if (map.getSource(HIGHLIGHT_SOURCE_ID)) {
    map.removeSource(HIGHLIGHT_SOURCE_ID);
    changed = true;
  }
  return changed;
}
