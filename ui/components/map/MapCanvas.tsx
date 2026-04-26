"use client";

import { useEffect, useRef } from "react";
import maplibregl, {
  type Map as MapLibreMap,
  AttributionControl,
  GeolocateControl,
  NavigationControl,
  ScaleControl,
} from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { registerPmtilesProtocol } from "@/lib/map/protocol";
import { DEFAULT_VIEWPORT, useMapStore } from "@/lib/state/map";
import { useUrlViewport } from "@/lib/map/use-url-viewport";
import { useStyleUrl } from "@/lib/api/hooks";
import { useMessages } from "@/lib/i18n/provider";
import { registerPoiIcons } from "@/lib/map/poi-icons";

/**
 * Props forwarded from `MapView`. Every option has a sensible default so
 * the common case (`<MapCanvas />`) renders the configured style with the
 * Google-Maps-style control layout.
 */
export interface MapCanvasProps {
  /** Which named style on the gateway to request. Defaults to `light`. */
  styleName?: "light" | "dark" | "print";
  /** If true, disables the built-in NavigationControl + ScaleControl. */
  hideControls?: boolean;
  /** If true, disables the GeolocateControl (mobile sensors). */
  hideGeolocate?: boolean;
  /** If true, enables the 3D-buildings + terrain setup. */
  enable3DBuildings?: boolean;
  /** Optional hook for test injection; called once the map is constructed. */
  onReady?: (map: MapLibreMap) => void;
}

/**
 * Inner MapLibre mount point. Owns the map instance lifetime, wires store
 * subscriptions, and re-attaches the style when region or theme changes.
 * This module must be imported with `ssr: false` (see `MapView.tsx`)
 * because MapLibre touches `window` + `document` at import time.
 */
export function MapCanvas({
  styleName = "light",
  hideControls = false,
  hideGeolocate = false,
  enable3DBuildings = false,
  onReady,
}: MapCanvasProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const mapRef = useRef<MapLibreMap | null>(null);
  const has3DRef = useRef(false);

  const setViewport = useMapStore((s) => s.setViewport);
  const setMap = useMapStore((s) => s.setMap);
  const setActiveRegion = useMapStore((s) => s.setActiveRegion);
  const setPendingClick = useMapStore((s) => s.setPendingClick);
  const setPendingContextmenu = useMapStore((s) => s.setPendingContextmenu);
  const activeRegion = useMapStore((s) => s.activeRegion);

  const { initial, commit } = useUrlViewport(DEFAULT_VIEWPORT);
  const { locale } = useMessages();
  const styleUrl = useStyleUrl(activeRegion, styleName, locale);

  // Hydrate activeRegion from the URL the first time we read it.
  useEffect(() => {
    if (initial?.region) setActiveRegion(initial.region);
    // Only run on the initial resolution — subsequent URL changes flow
    // through `commit` in the other direction.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial?.region]);

  // Core map-init effect. Runs once `initial` is available; re-runs are
  // avoided because MapLibre instances are expensive.
  useEffect(() => {
    if (!containerRef.current || mapRef.current || !initial) return;

    registerPmtilesProtocol();

    const map = new maplibregl.Map({
      container: containerRef.current,
      style: styleUrl,
      center: [initial.viewport.lon, initial.viewport.lat],
      zoom: initial.viewport.zoom,
      bearing: initial.viewport.bearing,
      pitch: initial.viewport.pitch,
      attributionControl: false,
      hash: false,
      transformRequest: (reqUrl) => {
        if (reqUrl.startsWith("/") && typeof window !== "undefined") {
          return { url: window.location.origin + reqUrl };
        }
        return { url: reqUrl };
      },
    });

    if (!hideControls) {
      map.addControl(
        new NavigationControl({ visualizePitch: true, showCompass: true }),
        "top-right",
      );
    }
    // Scale + attribution always present — the FabStack replaces the
    // navigation control but the chrome at bottom-right (the m/km
    // scale and "© OpenStreetMap" attribution from `2.png`) is
    // independent of `hideControls`.
    map.addControl(
      new ScaleControl({ maxWidth: 120, unit: "metric" }),
      "bottom-right",
    );
    map.addControl(
      // Inline text strip ("© OpenStreetMap …") next to the scale,
      // matching `7.png`'s bottom-right footer. compact:false keeps
      // the attribution always-visible instead of the hover-to-expand
      // "i" icon.
      new AttributionControl({ compact: false }),
      "bottom-right",
    );
    if (!hideGeolocate) {
      map.addControl(
        new GeolocateControl({
          positionOptions: { enableHighAccuracy: true },
          trackUserLocation: true,
        }),
        "top-right",
      );
    }

    const onMoveEnd = () => {
      const center = map.getCenter();
      const viewport = {
        lat: center.lat,
        lon: center.lng,
        zoom: map.getZoom(),
        bearing: map.getBearing(),
        pitch: map.getPitch(),
      };
      setViewport(viewport);
      commit({ viewport, region: useMapStore.getState().activeRegion });
    };
    const onClick = (e: maplibregl.MapMouseEvent) => {
      setPendingClick({
        lngLat: { lng: e.lngLat.lng, lat: e.lngLat.lat },
        point: { x: e.point.x, y: e.point.y },
        timestamp: Date.now(),
      });
    };
    const onContextMenu = (e: maplibregl.MapMouseEvent) => {
      e.preventDefault();
      setPendingContextmenu({
        lngLat: { lng: e.lngLat.lng, lat: e.lngLat.lat },
        point: { x: e.point.x, y: e.point.y },
        timestamp: Date.now(),
      });
    };
    const onStyleLoad = () => {
      setMap(map);
      if (enable3DBuildings) install3DBuildings(map, has3DRef);
      registerPoiIcons(map);
      onReady?.(map);
    };

    map.on("moveend", onMoveEnd);
    map.on("click", onClick);
    map.on("contextmenu", onContextMenu);
    map.on("load", onStyleLoad);
    mapRef.current = map;

    return () => {
      map.off("moveend", onMoveEnd);
      map.off("click", onClick);
      map.off("contextmenu", onContextMenu);
      map.off("load", onStyleLoad);
      setMap(null);
      map.remove();
      mapRef.current = null;
      has3DRef.current = false;
    };
    // The other deps are deliberately omitted: we reinit the map only on
    // initial-URL resolution. Style changes are applied via setStyle below.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [initial, hideControls, hideGeolocate]);

  // Swap style when region or theme changes — cheaper than reconstructing.
  useEffect(() => {
    const map = mapRef.current;
    if (!map) return;
    if (map.getStyle()?.sprite === undefined && !styleUrl) return;
    map.setStyle(styleUrl, { diff: false });
    has3DRef.current = false;
    const onStyleReload = () => {
      if (enable3DBuildings) install3DBuildings(map, has3DRef);
      registerPoiIcons(map);
    };
    map.once("styledata", onStyleReload);
    return () => {
      map.off("styledata", onStyleReload);
    };
  }, [styleUrl, enable3DBuildings]);

  return (
    <div
      ref={containerRef}
      className="absolute inset-0 h-full w-full"
      aria-label="Interactive map"
      role="region"
    />
  );
}

/**
 * Opt-in 3D-buildings + hillshading. Added only when `map.showBuildings3D`
 * is enabled; guarded by a ref so repeated style loads don't stack layers.
 */
function install3DBuildings(
  map: MapLibreMap,
  flagRef: { current: boolean },
): void {
  if (flagRef.current) return;
  const style = map.getStyle();
  if (!style) return;
  const hasBuildingSrc = Object.keys(style.sources ?? {}).some(
    (id) => id === "openmaptiles" || id === "protomaps" || id === "vector",
  );
  if (!hasBuildingSrc) return;
  // We intentionally don't assume a specific source id; 3D buildings
  // require a `building` layer whose source is installed by the server-
  // side style. If one exists already, add a fill-extrusion equivalent.
  const existing = style.layers?.find((l) => l.id === "buildings-3d");
  if (existing) {
    flagRef.current = true;
    return;
  }
  // No-op default: feature gated until the server-side style ships the
  // canonical building source. When it does, this is where to hook in.
  flagRef.current = true;
}
