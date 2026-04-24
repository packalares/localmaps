"use client";

import { useEffect } from "react";
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

  const styleName =
    props.styleName ?? (resolvedTheme === "dark" ? "dark" : "light");

  return <MapView {...props} styleName={styleName} />;
}

export default MainMap;
