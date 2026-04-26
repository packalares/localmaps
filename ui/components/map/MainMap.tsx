"use client";

import { useEffect, useRef } from "react";
import { useTheme } from "@/components/providers/theme";
import { MapView, type MapViewProps } from "./MapView";
import { useRegionsSync } from "@/lib/api/hooks";
import { useMapStore } from "@/lib/state/map";
import { toCanonicalRegionKey } from "@/lib/map/region-key";

/**
 * Top-level, ready-to-mount map surface used by `app/page.tsx`.
 *
 * Responsibilities this composer pulls together:
 * - Kick off `/api/regions` polling so the store's `installedRegions`
 *   stays fresh (feeds `RegionSwitcher` + future panels).
 * - Auto-select a sensible default region on first load when none is
 *   present in the URL: pick the first ready region alphabetically.
 * - Map the current theme (`light` / `dark`) to the style name, falling
 *   back to `light` so anonymous first-paint always renders something.
 *
 * All live event wiring (click, contextmenu, moveend, URL sync) happens
 * inside `MapCanvas`; this component merely assembles the inputs.
 */
export interface MainMapProps extends Omit<MapViewProps, "styleName"> {
  /** Override the theme-to-style mapping (mostly useful for tests). */
  styleName?: MapViewProps["styleName"];
}

export function MainMap(props: MainMapProps) {
  const { resolvedTheme } = useTheme();
  useRegionsSync();

  const installedRegions = useMapStore((s) => s.installedRegions);
  const activeRegion = useMapStore((s) => s.activeRegion);
  const setActiveRegion = useMapStore((s) => s.setActiveRegion);
  const map = useMapStore((s) => s.map);

  // Auto-select a ready region on first load if the URL didn't pick one.
  useEffect(() => {
    if (activeRegion) return;
    const ready = installedRegions
      .filter((r) => r.state === "ready")
      .sort((a, b) => a.name.localeCompare(b.name));
    if (ready.length > 0) {
      setActiveRegion(toCanonicalRegionKey(ready[0]!.name));
    }
  }, [activeRegion, installedRegions, setActiveRegion]);

  // Auto-pan/zoom to the active region's bbox on first load + every
  // region switch. Skipped when the URL already carried a viewport
  // hash (the hash wins so deep-links land where the user expects).
  // Falls back silently when the region row has no bbox (the worker
  // populates it on install — older installs without bbox keep the
  // previous default-viewport behaviour).
  const lastFittedRegion = useRef<string | null>(null);
  const initialUrlHasHash = useRef<boolean>(false);
  useEffect(() => {
    if (typeof window === "undefined") return;
    initialUrlHasHash.current = window.location.hash.length > 1;
  }, []);
  useEffect(() => {
    if (!map) return;
    if (!activeRegion) return;
    // Don't fight the URL hash on first paint.
    if (
      lastFittedRegion.current === null &&
      initialUrlHasHash.current
    ) {
      lastFittedRegion.current = activeRegion;
      return;
    }
    if (lastFittedRegion.current === activeRegion) return;
    const region = installedRegions.find(
      (r) => toCanonicalRegionKey(r.name) === activeRegion,
    );
    const bbox = region?.bbox;
    if (
      bbox &&
      bbox.length === 4 &&
      bbox.every((v) => Number.isFinite(v))
    ) {
      try {
        map.fitBounds(
          [
            [bbox[0], bbox[1]],
            [bbox[2], bbox[3]],
          ],
          { padding: 40, animate: true, duration: 600 },
        );
      } catch {
        /* jsdom / unmounted map — silently skip. */
      }
    }
    lastFittedRegion.current = activeRegion;
  }, [map, activeRegion, installedRegions]);

  const styleName =
    props.styleName ?? (resolvedTheme === "dark" ? "dark" : "light");

  // MapLibre's built-in NavigationControl + GeolocateControl are replaced
  // by `FabStack` in `app/page.tsx` (which renders a single, properly
  // spaced right-rail à la Google Maps). Disable the natives so we don't
  // render two overlapping stacks.
  return (
    <MapView
      hideControls
      hideGeolocate
      {...props}
      styleName={styleName}
    />
  );
}

export default MainMap;
