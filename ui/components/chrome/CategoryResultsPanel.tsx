"use client";

import { useCallback, useMemo } from "react";
import { Crosshair, X } from "lucide-react";
import type { GeocodeResult, Poi } from "@/lib/api/schemas";
import { ResultCard } from "@/components/search/ResultCard";
import { useMapStore, type PoiCategory } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { useDirectionsStore } from "@/lib/state/directions";
import { CATEGORY_DESCRIPTORS } from "./category-descriptors";
import { cn } from "@/lib/utils";

/**
 * Side-panel companion to the top POI chip row. Renders when the user
 * clicks a category chip — replaces the old absolute-positioned dropdown
 * that lived under the search bar. Layout mirrors `DirectionsPanel`:
 *
 *   ┌──────────────────────────────────────────┐
 *   │  Header: chip label + close X            │
 *   │  "Directions from your location" CTA     │
 *   │  ──────────────────────────────────────  │
 *   │  ResultCard  (icon + name + secondary)   │
 *   │  ResultCard                              │
 *   │  ...                                     │
 *   └──────────────────────────────────────────┘
 *
 * Clicking a row centers the map (`flyTo` zoom 16) and sets
 * `selectedFeature` so the bottom point/POI info card appears. The
 * panel stays open until the chip is cleared.
 */
export function CategoryResultsPanel() {
  const activeCategoryChip = useMapStore((s) => s.activeCategoryChip);
  const categorySearchResults = useMapStore((s) => s.categorySearchResults);
  const map = useMapStore((s) => s.map);
  const viewport = useMapStore((s) => s.viewport);
  const setViewport = useMapStore((s) => s.setViewport);
  const closeCategoryResults = useMapStore((s) => s.closeCategoryResults);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  const setSelectedFeature = usePlaceStore((s) => s.setSelectedFeature);
  const setWaypointFromPoint = useDirectionsStore(
    (s) => s.setWaypointFromPoint,
  );
  const setWaypoint = useDirectionsStore((s) => s.setWaypoint);

  const focus = useMemo(
    () => ({ lat: viewport.lat, lon: viewport.lon }),
    [viewport.lat, viewport.lon],
  );

  // Closing = full reset: the chip pill goes inactive, markers
  // disappear, the search-bar (which mirrors the chip label) clears
  // automatically, and the rail slides back to its idle state.
  const handleClose = useCallback(() => {
    closeCategoryResults();
  }, [closeCategoryResults]);

  const handlePick = useCallback(
    (poi: Poi) => {
      // Center the map on the result — pan only, no zoom change
      // (Change 6). Falls back to setViewport with the SAME zoom in
      // test envs where the live map is missing.
      const currentZoom = (() => {
        try {
          return map ? map.getZoom() : viewport.zoom;
        } catch {
          return viewport.zoom;
        }
      })();
      if (map) {
        try {
          map.flyTo({
            center: [poi.center.lon, poi.center.lat],
            zoom: currentZoom,
            essential: true,
          });
        } catch {
          /* ignore — viewport sync below covers the fallback */
        }
      }
      setViewport({
        lat: poi.center.lat,
        lon: poi.center.lon,
        zoom: currentZoom,
        bearing: viewport.bearing,
        pitch: viewport.pitch,
      });
      setSelectedFeature({
        kind: "poi",
        id: poi.id,
        lat: poi.center.lat,
        lon: poi.center.lon,
        name: poi.label,
      });
    },
    [
      map,
      setViewport,
      viewport.bearing,
      viewport.pitch,
      viewport.zoom,
      setSelectedFeature,
    ],
  );

  const handleUseMyLocation = useCallback(() => {
    if (typeof navigator === "undefined" || !navigator.geolocation) {
      // Geolocation unavailable; still flip the user into the
      // directions panel so they can pick a starting point manually.
      openLeftRail("directions");
      return;
    }
    try {
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          setWaypointFromPoint(
            0,
            { lng: pos.coords.longitude, lat: pos.coords.latitude },
            "Your location",
          );
          // Clear the destination slot so the user picks one from the
          // result list / panel that follows.
          setWaypoint(1, { label: "", lngLat: null });
          openLeftRail("directions");
        },
        () => {
          // Permission denied / timeout: still surface the directions
          // panel — the user can type a starting point.
          openLeftRail("directions");
        },
        { enableHighAccuracy: false, timeout: 8000, maximumAge: 60_000 },
      );
    } catch {
      openLeftRail("directions");
    }
  }, [openLeftRail, setWaypoint, setWaypointFromPoint]);

  if (!activeCategoryChip) return null;

  const label = labelForCategory(activeCategoryChip);

  return (
    <section
      aria-label={`${label} results`}
      className="flex h-full min-h-0 flex-col"
    >
      <header className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
        <h2 className="truncate text-base font-semibold text-foreground">
          {label}
        </h2>
        <button
          type="button"
          onClick={handleClose}
          aria-label="Close results"
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center rounded-full text-muted-foreground",
            "hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      </header>

      <div className="border-b border-border px-3 py-2">
        <button
          type="button"
          onClick={handleUseMyLocation}
          className={cn(
            "flex w-full items-center gap-3 rounded-md px-2 py-2 text-left text-sm font-medium",
            "text-foreground hover:bg-muted focus:outline-none focus-visible:bg-muted",
          )}
        >
          <span
            aria-hidden="true"
            className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
          >
            <Crosshair className="h-4 w-4" />
          </span>
          <span className="flex flex-col">
            <span>Directions from your location</span>
            <span className="text-xs font-normal text-muted-foreground">
              Pick a destination from the list
            </span>
          </span>
        </button>
      </div>

      <div
        role="listbox"
        aria-label={`${label} list`}
        className="flex min-h-0 flex-1 flex-col overflow-y-auto px-1 py-1"
      >
        {categorySearchResults.length === 0 ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            No results in this area. Pan the map and click the chip again.
          </p>
        ) : (
          categorySearchResults.map((poi) => (
            <ResultCard
              key={poi.id}
              result={poiToGeocodeResult(poi)}
              origin={focus}
              onSelect={() => handlePick(poi)}
            />
          ))
        )}
      </div>
    </section>
  );
}

/**
 * Adapt a POI to the GeocodeResult shape that ResultCard expects. Only
 * the fields the card actually reads are populated; the rest stay
 * undefined, which the schema allows.
 */
function poiToGeocodeResult(poi: Poi): GeocodeResult {
  return {
    id: poi.id,
    label: poi.label,
    center: { lat: poi.center.lat, lon: poi.center.lon },
    category: poi.category ?? undefined,
    confidence: 1,
  };
}

function labelForCategory(cat: PoiCategory): string {
  const found = CATEGORY_DESCRIPTORS.find((c) => c.key === cat);
  return found?.label ?? cat;
}
