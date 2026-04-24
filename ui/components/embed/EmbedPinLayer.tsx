"use client";

import { useEffect, useRef, useState } from "react";
import type { Map as MapLibreMap, Popup } from "maplibre-gl";
import maplibregl from "maplibre-gl";
import type { EmbedPin } from "./params";

/**
 * Renders a single pin on top of a MapLibre map. The pin is represented as
 * a GeoJSON point source + a circle layer so we don't need to ship sprite
 * assets into the embed bundle. A label click opens a compact popup that
 * shows the optional label text.
 *
 * The component is idempotent: on every pin change it removes the existing
 * source/layer (if any) before re-adding. Unmount tears everything down.
 */
export interface EmbedPinLayerProps {
  /** The map the pin is drawn on. `null` while the map is still loading. */
  map: MapLibreMap | null;
  /** The pin to render, or `null` to hide it. */
  pin: EmbedPin | null;
}

const SOURCE_ID = "localmaps-embed-pin";
const LAYER_ID = "localmaps-embed-pin-layer";

export function EmbedPinLayer({ map, pin }: EmbedPinLayerProps) {
  const popupRef = useRef<Popup | null>(null);
  const [, setReady] = useState(false);

  // Install/remove the pin whenever the inputs change.
  useEffect(() => {
    if (!map) return;
    const install = () => {
      if (!pin) {
        removePin(map);
        setReady(false);
        return;
      }
      installPin(map, pin);
      setReady(true);
    };

    // The style may still be loading when `map` first becomes available;
    // MapLibre throws on source-add before `style.load`. Wait it out.
    if (map.isStyleLoaded()) install();
    else map.once("load", install);

    return () => {
      map.off("load", install);
      if (popupRef.current) {
        popupRef.current.remove();
        popupRef.current = null;
      }
      removePin(map);
    };
    // We intentionally depend on the pin's scalar fields rather than the
    // whole object; a new object literal with identical contents must not
    // re-install the pin.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [map, pin?.lat, pin?.lon, pin?.label]);

  // Click handler: open a popup with the label (if any). Purely local — no
  // fetch, no store mutation; that's the contract for embed interactions.
  useEffect(() => {
    if (!map || !pin) return;
    const handler = () => {
      if (popupRef.current) popupRef.current.remove();
      const text = pin.label ?? formatCoord(pin.lat, pin.lon);
      const popup = new maplibregl.Popup({ closeButton: true, offset: 16 })
        .setLngLat([pin.lon, pin.lat])
        .setText(text)
        .addTo(map);
      popupRef.current = popup;
    };
    map.on("click", LAYER_ID, handler);
    return () => {
      map.off("click", LAYER_ID, handler);
    };
    // Same reasoning as the install effect above.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [map, pin?.lat, pin?.lon, pin?.label]);

  return null;
}

function installPin(map: MapLibreMap, pin: EmbedPin): void {
  removePin(map);
  map.addSource(SOURCE_ID, {
    type: "geojson",
    data: {
      type: "Feature",
      properties: { label: pin.label ?? "" },
      geometry: { type: "Point", coordinates: [pin.lon, pin.lat] },
    },
  });
  map.addLayer({
    id: LAYER_ID,
    type: "circle",
    source: SOURCE_ID,
    paint: {
      "circle-radius": 8,
      "circle-color": "#ef4444",
      "circle-stroke-color": "#ffffff",
      "circle-stroke-width": 2,
    },
  });
}

function removePin(map: MapLibreMap): void {
  if (map.getLayer(LAYER_ID)) map.removeLayer(LAYER_ID);
  if (map.getSource(SOURCE_ID)) map.removeSource(SOURCE_ID);
}

function formatCoord(lat: number, lon: number): string {
  return `${lat.toFixed(5)}, ${lon.toFixed(5)}`;
}

export default EmbedPinLayer;
