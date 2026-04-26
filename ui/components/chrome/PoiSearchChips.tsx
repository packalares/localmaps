"use client";

import { useEffect } from "react";
import maplibregl from "maplibre-gl";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { POI_CATEGORIES, useMapStore } from "@/lib/state/map";
import { applyAllPoiVisibility } from "@/lib/poi/visibility";
import { useCategorySearch } from "@/lib/api/hooks";
import { usePlaceStore } from "@/lib/state/place";
import { CATEGORY_DESCRIPTORS, descriptorFor } from "./category-descriptors";
import { makeChipMarkerElement } from "./marker-elements";

/**
 * Google-Maps "search-here-for-X" chip row. Each chip is a search
 * trigger — not a visibility toggle. Clicking one fires a category
 * search constrained to the current viewport bbox, opens the side
 * panel with the result list, and pins the returned POIs on the map.
 * Clicking the active chip clears markers + the side panel.
 *
 * Visibility lives elsewhere — `LayersCard`'s popover owns the
 * per-category checklist. This component still drives the MapLibre
 * style sync (`applyAllPoiVisibility`) because the effect only cares
 * about the store and needs to keep running regardless of which chrome
 * surface owns the UI.
 *
 * Layout: the always-on side-effects (visibility sync, category-search
 * fetch, marker overlay) live on a sibling `<PoiSearchEffects />` that
 * renders no DOM. The chip-row UI itself unmounts when a chip is active
 * (it visually overlaps the side panel — Google Maps hides it then) or
 * the result panel is open. Splitting them lets the markers keep
 * rendering even when the row is hidden.
 */

/** Minimum zoom at which POIs start rendering in the style. */
export const POI_VISIBILITY_MIN_ZOOM = 12;

export interface PoiSearchChipsProps {
  /** Reserved for tests; the chip row is always visible at runtime. */
  forceVisible?: boolean;
}

/** Serialise the live map's viewport into the `minLon,minLat,maxLon,maxLat` */
/** payload expected by `GET /api/pois`. Returns null if the map is absent. */
function bboxFromMap(map: maplibregl.Map | null): string | null {
  if (!map) return null;
  try {
    const b = map.getBounds();
    return `${b.getWest()},${b.getSouth()},${b.getEast()},${b.getNorth()}`;
  } catch {
    return null;
  }
}

/**
 * Always-on side-effect host. Renders nothing; subscribes to map +
 * activeCategoryChip + categorySearchBbox and:
 *   1. Keeps `applyAllPoiVisibility` in sync.
 *   2. Fires the category search against the FROZEN bbox.
 *   3. Mirrors results into the Zustand slice.
 *   4. Drops a custom HTML marker per result.
 *
 * Hoisted out of the chip-row component so the row can unmount without
 * dropping the markers. The effects watch the store, so the row's
 * visibility doesn't matter to them.
 */
function PoiSearchEffects() {
  const map = useMapStore((s) => s.map);
  const poiVisibility = useMapStore((s) => s.poiVisibility);
  const activeCategoryChip = useMapStore((s) => s.activeCategoryChip);
  const categorySearchBbox = useMapStore((s) => s.categorySearchBbox);
  const categorySearchResults = useMapStore((s) => s.categorySearchResults);
  const setCategorySearchResults = useMapStore(
    (s) => s.setCategorySearchResults,
  );
  const setSelectedFeature = usePlaceStore((s) => s.setSelectedFeature);

  // Category search. Enabled only while a chip is active; switching
  // chips recycles the query key so TanStack Query drops the stale set.
  // The bbox is the FROZEN snapshot captured at chip-click time — pan
  // and zoom no longer refire the query (Change 7). The user has to
  // re-click the chip to search the new viewport.
  const search = useCategorySearch({
    bbox: categorySearchBbox,
    category: activeCategoryChip,
    size: 50,
    enabled: !!activeCategoryChip,
  });

  // Keep the live style in sync whenever the visibility map changes or
  // a new map instance becomes available.
  useEffect(() => {
    if (!map) return;
    applyAllPoiVisibility(map, poiVisibility);
  }, [map, poiVisibility]);

  useEffect(() => {
    if (!map) return;
    const onStyleData = () => {
      applyAllPoiVisibility(map, poiVisibility);
    };
    map.on("styledata", onStyleData);
    return () => {
      map.off("styledata", onStyleData);
    };
  }, [map, poiVisibility]);

  // Mirror TanStack Query's latest result into the Zustand slice so
  // sibling components (and tests) can read a stable value without
  // going through the hook.
  useEffect(() => {
    if (!activeCategoryChip) {
      setCategorySearchResults([]);
      return;
    }
    if (search.data) {
      setCategorySearchResults(search.data.pois);
    }
  }, [activeCategoryChip, search.data, setCategorySearchResults]);

  // Drop custom HTML markers on every POI in the current result set.
  // Each marker owns its own click handler that publishes the POI as
  // `selectedFeature` and stops propagation so the map's primary click
  // pipeline doesn't fire a competing "drop pin" event behind it.
  // The visual matches Google Maps' chip-search results in `3.png`:
  // red teardrop with the category glyph centred, and the POI name
  // shown in a small white pill to the right of the marker.
  useEffect(() => {
    if (!map) return;
    if (!activeCategoryChip) return;
    const desc = descriptorFor(activeCategoryChip);
    const markers: maplibregl.Marker[] = [];
    const cleanups: Array<() => void> = [];
    for (const poi of categorySearchResults) {
      try {
        const el = makeChipMarkerElement({
          iconPath: desc.iconPath,
          label: poi.label,
          // Per-descriptor tint so chip + markers share colour identity
          // (Food=orange, Hotels=violet, etc). The pill in the chip row
          // also uses `desc.color`.
          color: desc.color,
        });
        const onClick = (ev: Event) => {
          // Block the click from reaching MapCanvas's handler so the
          // generic "dropped pin" path doesn't run on top of the chip
          // marker.
          ev.stopPropagation();
          setSelectedFeature({
            kind: "poi",
            id: poi.id,
            lat: poi.center.lat,
            lon: poi.center.lon,
            name: poi.label,
          });
        };
        el.addEventListener("click", onClick);
        cleanups.push(() => el.removeEventListener("click", onClick));
        // Anchor at `bottom` so the tip of the teardrop sits on the
        // POI coordinate (the previous round disc anchored centre).
        const m = new maplibregl.Marker({ element: el, anchor: "bottom" })
          .setLngLat([poi.center.lon, poi.center.lat])
          .addTo(map);
        markers.push(m);
      } catch {
        // jsdom / test env may not support full Marker API.
      }
    }
    return () => {
      for (const cleanup of cleanups) cleanup();
      for (const m of markers) {
        try {
          m.remove();
        } catch {
          /* ignore */
        }
      }
    };
  }, [map, activeCategoryChip, categorySearchResults, setSelectedFeature]);

  return null;
}

/**
 * Top-level wrapper. Mounts `<PoiSearchEffects />` (always) and the
 * chip-row UI (conditionally — hidden while a chip is active or a
 * result panel is open).
 */
export function PoiSearchChips(_props: PoiSearchChipsProps = {}) {
  const map = useMapStore((s) => s.map);
  const activeRegion = useMapStore((s) => s.activeRegion);
  const activeCategoryChip = useMapStore((s) => s.activeCategoryChip);
  const runCategorySearch = useMapStore((s) => s.runCategorySearch);
  const leftRailTab = useMapStore((s) => s.leftRailTab);
  const viewport = useMapStore((s) => s.viewport);

  // POIs only render in the basemap above zoom 12, but the chip
  // search runs against the active region's bbox via pelias — works
  // at any zoom. We don't block low-zoom clicks anymore (the previous
  // toast-based hint never showed because the toast handler isn't
  // mounted, so users saw "click does nothing").
  const belowMinZoom = false;

  // Hide the row whenever a chip is active OR the result panel is open
  // (free-text results / chip results). Avoids the row overlapping the
  // 400px side panel — Google Maps does the same (Change 2). The
  // sibling `<PoiSearchEffects />` is always mounted so markers and
  // the active-chip query keep ticking even while the row is hidden.
  // Hide the chip row whenever ANY left-side panel is open (recents,
  // saved, results, categoryResults, directions, place) OR a chip is
  // active. The "search" tab (closed/idle state) is the only one that
  // shows the chips.
  const rowHidden =
    activeCategoryChip !== null ||
    (leftRailTab !== "search");

  return (
    <>
      <PoiSearchEffects />
      {rowHidden ? null : (
        <TooltipProvider delayDuration={400}>
          <div className="relative w-full">
            <div
              role="group"
              aria-label="Search POIs by category"
              className="scrollbar-none pointer-events-auto flex max-w-full items-center gap-2 overflow-x-auto"
            >
              {CATEGORY_DESCRIPTORS.map((desc) => {
                const active = activeCategoryChip === desc.key;
                const Icon = desc.icon;
                const disabled = false;
                // Tint the active chip with the descriptor's hex; the
                // matching markers also use `desc.color` so chip and
                // pins share an identity.
                const activeStyle = active
                  ? {
                      backgroundColor: desc.color,
                      borderColor: desc.color,
                      color: "#ffffff",
                    }
                  : undefined;
                return (
                  <Tooltip key={desc.key}>
                    <TooltipTrigger asChild>
                      <button
                        type="button"
                        onClick={() => {
                          // Capture the live viewport bbox at the moment
                          // of activation; the store freezes it for the
                          // chip's lifetime so subsequent pan/zoom does
                          // NOT cause the query to refire.
                          const liveBbox = bboxFromMap(map);
                          runCategorySearch(desc.key, liveBbox);
                        }}
                        data-active={active ? "true" : "false"}
                        data-category={desc.key}
                        aria-label={
                          active
                            ? `Clear ${desc.label} results`
                            : `Search ${desc.label} in this area`
                        }
                        style={activeStyle}
                        className={cn(
                          "inline-flex h-8 shrink-0 items-center gap-1.5 whitespace-nowrap rounded-full border px-3 text-[13px] font-medium shadow-sm",
                          "transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                          active
                            ? "border-transparent text-white hover:opacity-90"
                            : "border-black/10 bg-white text-neutral-700 hover:bg-neutral-50 dark:border-white/10 dark:bg-neutral-900 dark:text-neutral-200 dark:hover:bg-neutral-800",
                          disabled && "opacity-70",
                        )}
                      >
                        <Icon className="h-4 w-4" aria-hidden="true" />
                        <span>{desc.short}</span>
                      </button>
                    </TooltipTrigger>
                    <TooltipContent side="bottom" align="center">
                      <span className="block">{desc.label}</span>
                      <span className="block text-[10px] opacity-80">
                        {activeRegion === null
                          ? "Install a region first"
                          : belowMinZoom && !active
                          ? "Zoom in to search"
                          : "Click to search this area"}
                      </span>
                    </TooltipContent>
                  </Tooltip>
                );
              })}
            </div>
          </div>
        </TooltipProvider>
      )}
    </>
  );
}

/** Re-export the canonical list so tests importing from the old surface
 *  still compile after the rename. */
export { POI_CATEGORIES };
