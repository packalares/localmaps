"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import maplibregl, {
  type Map as MapLibreMap,
  NavigationControl,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Minus, Plus } from "lucide-react";
import { apiUrl } from "@/lib/env";
import { registerPmtilesProtocol } from "@/lib/map/protocol";
import { EmbedPinLayer } from "./EmbedPinLayer";
import type { EmbedPin } from "./params";

/**
 * Client-only MapLibre canvas used inside `/embed`. Deliberately separate
 * from `MapCanvas` for the main viewer because the embed surface must:
 *
 *   - not read the URL hash at startup (the gateway passes lat/lon/zoom
 *     via query params which the server component already parsed);
 *   - not subscribe to `/api/regions` polling (no cookies, no auth);
 *   - not mount the left rail, FABs, or right-click menu;
 *   - still persist intra-iframe pans to the hash so reloads inside the
 *     iframe survive.
 *
 * Keep this file under 250 lines per the agent rules.
 */
export interface EmbedMapInnerProps {
  center: { lat: number; lon: number };
  zoom: number;
  styleName: "light" | "dark" | "print";
  region: string | null;
  pin: EmbedPin | null;
}

function buildStyleUrl(
  region: string | null,
  styleName: EmbedMapInnerProps["styleName"],
): string {
  const base = apiUrl(`/api/styles/${styleName}.json`);
  if (!region) return base;
  const qs = new URLSearchParams({ region }).toString();
  return `${base}?${qs}`;
}

export function EmbedMapInner({
  center,
  zoom,
  styleName,
  region,
  pin,
}: EmbedMapInnerProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const [live, setLive] = useState<MapLibreMap | null>(null);

  const styleUrl = useMemo(
    () => buildStyleUrl(region, styleName),
    [region, styleName],
  );

  // Construct the MapLibre instance once; re-running is expensive.
  useEffect(() => {
    if (!containerRef.current || mapRef.current) return;
    registerPmtilesProtocol();
    const map = new maplibregl.Map({
      container: containerRef.current,
      style: styleUrl,
      center: [center.lon, center.lat],
      zoom,
      attributionControl: false,
      cooperativeGestures: true, // Hint to embed users: ctrl+scroll to zoom.
      hash: false,
    });
    // A built-in NavigationControl keeps the zoom +/- buttons accessible;
    // the compass stays hidden because embed users rarely rotate.
    map.addControl(
      new NavigationControl({ visualizePitch: false, showCompass: false }),
      "top-right",
    );

    const onLoad = () => setLive(map);
    map.on("load", onLoad);
    mapRef.current = map;

    return () => {
      map.off("load", onLoad);
      setLive(null);
      map.remove();
      mapRef.current = null;
    };
    // Re-initialising the map on every prop change defeats the purpose; we
    // treat the incoming values as the _initial_ viewport only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Swap the style in place when region/theme change after mount.
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    map.setStyle(styleUrl, { diff: false });
  }, [styleUrl]);

  const zoomIn = useCallback(() => {
    mapRef.current?.zoomIn({ duration: 200 });
  }, []);
  const zoomOut = useCallback(() => {
    mapRef.current?.zoomOut({ duration: 200 });
  }, []);

  return (
    <>
      <div
        ref={containerRef}
        className="absolute inset-0 h-full w-full"
        aria-label="Interactive map"
        role="region"
        data-testid="embed-map-canvas"
      />
      {/* Redundant custom controls — present even when MapLibre's own
          NavigationControl is hidden by a CSS override on small iframes. */}
      <div className="pointer-events-none absolute right-2 top-2 flex flex-col gap-1">
        <button
          type="button"
          onClick={zoomIn}
          aria-label="Zoom in"
          className="pointer-events-auto flex h-9 w-9 items-center justify-center rounded-md bg-chrome-surface/90 text-foreground shadow-chrome ring-1 ring-chrome-border hover:bg-muted"
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
        </button>
        <button
          type="button"
          onClick={zoomOut}
          aria-label="Zoom out"
          className="pointer-events-auto flex h-9 w-9 items-center justify-center rounded-md bg-chrome-surface/90 text-foreground shadow-chrome ring-1 ring-chrome-border hover:bg-muted"
        >
          <Minus className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>
      <EmbedPinLayer map={live} pin={pin} />
    </>
  );
}

export default EmbedMapInner;
