"use client";

import { useCallback, useMemo, useRef } from "react";
import { Loader2 } from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";
import {
  useGeocodeAutocomplete,
  useGeocodeSearch,
} from "@/lib/api/hooks";
import { useMapStore } from "@/lib/state/map";
import { usePlaceStore } from "@/lib/state/place";
import { useDebouncedValue } from "@/lib/search/debounce";
import { useKeyboardNav } from "@/lib/search/keyboard-nav";
import { cn } from "@/lib/utils";
import { ResultCard } from "./ResultCard";
import { RecentHistory, pushHistoryEntry } from "./RecentHistory";
import { useMessages } from "@/lib/i18n/provider";

export interface SearchPanelProps {
  /** Controlled query — typically mirrors the SearchBar input. */
  query: string;
  /**
   * When the user activates a result, the SearchBar gets told so the
   * pill text can mirror what was picked.
   */
  onResultSelected?: (result: GeocodeResult) => void;
  /** Debounce delay in ms. Operator-configurable. Default 300. */
  debounceMs?: number;
  /**
   * If true, the `query` is treated as a "submit" (Enter on the SearchBar),
   * triggering the fuller `/api/geocode/search` endpoint instead of
   * autocomplete.
   */
  fullSearch?: boolean;
  /** Optional locale forwarded to the autocomplete endpoint. */
  lang?: string;
  /** Upper bound on results displayed. Default 10 (openapi default). */
  limit?: number;
}

export function SearchPanel({
  query,
  onResultSelected,
  debounceMs = 300,
  fullSearch = false,
  lang,
  limit,
}: SearchPanelProps) {
  const debouncedQuery = useDebouncedValue(query, debounceMs);
  const trimmed = debouncedQuery.trim();
  const { t } = useMessages();

  const map = useMapStore((s) => s.map);
  const viewport = useMapStore((s) => s.viewport);
  const installedRegions = useMapStore((s) => s.installedRegions);

  const focus = useMemo(
    () => ({ lat: viewport.lat, lon: viewport.lon }),
    [viewport.lat, viewport.lon],
  );

  // Pick the right endpoint based on whether the user pressed Enter.
  const autocomplete = useGeocodeAutocomplete({
    q: trimmed,
    focus,
    limit,
    lang,
    enabled: !fullSearch && trimmed.length > 0,
  });
  const fullSearchQuery = useGeocodeSearch({
    q: trimmed,
    focus,
    limit,
    enabled: fullSearch && trimmed.length > 0,
  });

  const active = fullSearch ? fullSearchQuery : autocomplete;
  const results = active.data?.results ?? [];
  const isLoading = active.isLoading && trimmed.length > 0;

  const selectResult = useCallback(
    (result: GeocodeResult) => {
      pushHistoryEntry(result);
      onResultSelected?.(result);

      // Drive the standard PointInfoCard pin via the place store so
      // the user gets the same Google-style marker + info card they
      // see on map clicks. The bottom card derives its label from
      // `feature.name` (here = the result label) so no extra fetch.
      const placeStore = usePlaceStore.getState();
      placeStore.setSelectedFeature({
        kind: "poi",
        id: result.id,
        lat: result.center.lat,
        lon: result.center.lon,
        name: result.label,
        address: result.label,
      });

      if (map) {
        let z: number;
        try {
          z = map.getZoom();
        } catch {
          z = viewport.zoom;
        }
        try {
          map.flyTo({
            center: [result.center.lon, result.center.lat],
            zoom: z,
            essential: true,
          });
        } catch {
          // Map may have been torn down mid-click; skip silently.
        }
      }
    },
    [map, onResultSelected, viewport.zoom],
  );

  const listRef = useRef<HTMLDivElement | null>(null);

  const nav = useKeyboardNav<GeocodeResult>({
    items: results,
    onSelect: selectResult,
    onEscape: () => {
      // Blur focus so the user is back on the map.
      (document.activeElement as HTMLElement | null)?.blur?.();
    },
  });

  const showEmptyNoQuery = trimmed.length === 0;
  const showEmptyNoResults =
    !showEmptyNoQuery && !isLoading && results.length === 0 && !active.error;
  const showEmptyNoRegions = installedRegions.length === 0 && showEmptyNoQuery;

  return (
    <section
      className="flex h-full min-h-0 flex-col"
      aria-label="Search results"
      onKeyDown={(e) => nav.handleKeyDown(e)}
    >
      {/* Live region for count announcements. */}
      <div className="sr-only" aria-live="polite" role="status">
        {trimmed.length === 0
          ? ""
          : isLoading
          ? "Searching"
          : results.length === 0
          ? "No matches"
          : `${results.length} result${results.length === 1 ? "" : "s"}`}
      </div>

      {trimmed.length > 0 ? (
        <header className="flex items-center justify-between px-3 pb-2 pt-1 text-xs text-muted-foreground">
          <span className="truncate">
            <span className="text-foreground">{trimmed}</span>
            {results.length > 0 ? (
              <span>
                {" "}· {results.length} result{results.length === 1 ? "" : "s"}
              </span>
            ) : null}
          </span>
          {isLoading ? (
            <Loader2
              className="h-3.5 w-3.5 animate-spin"
              aria-label="Searching"
            />
          ) : null}
        </header>
      ) : null}

      <div
        ref={listRef}
        role="listbox"
        aria-label="Search results list"
        className={cn("flex min-h-0 flex-1 flex-col overflow-y-auto")}
      >
        {results.length > 0 ? (
          <div className="flex flex-col gap-0.5 px-1 pb-1">
            {results.map((r, idx) => (
              <ResultCard
                key={r.id}
                id={`search-result-${idx}`}
                result={r}
                origin={focus}
                highlighted={idx === nav.highlightedIndex}
                onSelect={selectResult}
                onPointerOver={() => nav.setHighlightedIndex(idx)}
              />
            ))}
          </div>
        ) : null}

        {showEmptyNoResults ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            {t("search.empty.noResults")}
          </p>
        ) : null}

        {showEmptyNoQuery && !showEmptyNoRegions ? (
          <>
            <p className="px-4 pb-2 pt-4 text-sm text-muted-foreground">
              {t("search.empty.prompt")}
            </p>
            <RecentHistory onSelect={selectResult} origin={focus} />
          </>
        ) : null}

        {showEmptyNoRegions ? (
          <p className="px-4 py-6 text-sm text-muted-foreground">
            {t("search.empty.noRegions")}
          </p>
        ) : null}
      </div>
    </section>
  );
}
