"use client";

import { useCallback } from "react";
import { Clock } from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";
import { ResultCard } from "@/components/search/ResultCard";
import { clearHistory } from "@/components/search/RecentHistory";
import { useRecentHistory } from "@/lib/search/history";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { cn } from "@/lib/utils";

/**
 * Side-panel companion to the LeftIconRail's Recents button. Shows the
 * full localStorage-backed history with a "Clear all" CTA. Clicking a
 * row pans the map and surfaces the entry in the bottom info card via
 * `usePlaceStore.setSelectedFeature` — same flow as a dropdown recent.
 *
 * Replaces the prior `recents` LeftRail branch that re-rendered
 * `<SearchPanel>` pre-seeded with a stale result label.
 */
export function RecentsPanel() {
  const entries = useRecentHistory();
  const map = useMapStore((s) => s.map);
  const viewport = useMapStore((s) => s.viewport);
  const setViewport = useMapStore((s) => s.setViewport);
  const setSelectedFeature = usePlaceStore((s) => s.setSelectedFeature);

  const handleSelect = useCallback(
    (entry: GeocodeResult) => {
      // Surface the entry in the canonical place store so the bottom
      // info card opens.
      setSelectedFeature({
        kind: "poi",
        id: entry.id,
        lat: entry.center.lat,
        lon: entry.center.lon,
        name: entry.label,
        address: entry.label,
      });

      // Pan-only — preserve the user's zoom (Change 6).
      let z = viewport.zoom;
      try {
        if (map) z = map.getZoom();
      } catch {
        /* fall back to store zoom */
      }
      if (map) {
        try {
          map.flyTo({
            center: [entry.center.lon, entry.center.lat],
            zoom: z,
            essential: true,
          });
        } catch {
          /* ignore — viewport sync below covers fallback */
        }
      }
      setViewport({
        lat: entry.center.lat,
        lon: entry.center.lon,
        zoom: z,
        bearing: viewport.bearing,
        pitch: viewport.pitch,
      });
    },
    [map, setSelectedFeature, setViewport, viewport.bearing, viewport.pitch, viewport.zoom],
  );

  const handleClear = useCallback(() => {
    clearHistory();
  }, []);

  return (
    <section
      aria-label="Recent searches"
      className="flex h-full min-h-0 flex-col"
    >
      <header className="flex items-center justify-between gap-2 border-b border-border px-4 py-3">
        <div className="flex min-w-0 items-center gap-2">
          <Clock className="h-4 w-4 text-muted-foreground" aria-hidden="true" />
          <h2 className="truncate text-base font-semibold text-foreground">
            Recent searches
          </h2>
        </div>
        {entries.length > 0 ? (
          <button
            type="button"
            onClick={handleClear}
            className={cn(
              "inline-flex items-center rounded-full border border-border px-2.5 py-1 text-xs font-medium text-muted-foreground",
              "hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
            aria-label="Clear search history"
          >
            Clear all
          </button>
        ) : null}
      </header>

      <div
        role="listbox"
        aria-label="Recent searches list"
        className="flex min-h-0 flex-1 flex-col overflow-y-auto px-1 py-1"
      >
        {entries.length === 0 ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            No recent searches yet. Pick a place from the search bar to see
            it here.
          </p>
        ) : (
          entries.map((entry) => (
            <ResultCard
              key={entry.id}
              result={entry}
              origin={{ lat: viewport.lat, lon: viewport.lon }}
              onSelect={handleSelect}
            />
          ))
        )}
      </div>
    </section>
  );
}
