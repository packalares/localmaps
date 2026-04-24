"use client";

import type { Map as MapLibreMap } from "maplibre-gl";
import type { GeocodeResult } from "@/lib/api/schemas";
import { registerLayer, unregisterLayer } from "@/lib/map/layer-bus";

/**
 * Small helpers that mutate the map in response to a search selection.
 * Split out from `<SearchPanel>` so the component stays under the 250-
 * line ceiling and so the behaviour is independently unit-testable.
 */

export const SEARCH_PIN_LAYER_ID = "search-selected-pin";
const DEFAULT_FLY_ZOOM = 14;

/** Map category hints to a sensible zoom level. */
export function computeZoomForResult(result: GeocodeResult): number {
  const cat = result.category?.toLowerCase() ?? "";
  if (/country|nation/.test(cat)) return 5;
  if (/region|state|county|province/.test(cat)) return 7;
  if (/locality|city|town/.test(cat)) return 11;
  if (/borough|neighbourhood|suburb/.test(cat)) return 13;
  if (/street|road|way/.test(cat)) return 15;
  if (/address|house|venue|building/.test(cat)) return 17;
  return DEFAULT_FLY_ZOOM;
}

/**
 * Drop a pin layer at the picked centre. Uses the layer-bus
 * `registerLayer(map, id, source, layer)` signature; re-registering with
 * the same id replaces the source + layer in place.
 */
export function dropSearchPin(
  map: MapLibreMap,
  lat: number,
  lon: number,
): void {
  registerLayer(
    map,
    SEARCH_PIN_LAYER_ID,
    {
      type: "geojson",
      data: {
        type: "Feature",
        geometry: { type: "Point", coordinates: [lon, lat] },
        properties: {},
      },
    },
    {
      type: "circle",
      paint: {
        "circle-radius": 8,
        "circle-color": "#2563eb",
        "circle-stroke-color": "#ffffff",
        "circle-stroke-width": 2,
      },
    },
  );
}

/** Remove the pin layer. No-op if the layer was never registered. */
export function clearSearchPin(map: MapLibreMap): void {
  try {
    unregisterLayer(map, SEARCH_PIN_LAYER_ID);
  } catch {
    // ignore
  }
}
