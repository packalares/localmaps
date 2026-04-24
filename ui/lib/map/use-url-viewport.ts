"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  formatHash,
  parseHash,
  type MapViewport,
} from "@/lib/url-state";
import { isCanonicalRegionKey } from "./region-key";

/**
 * Two-way-binds a map viewport + active region to the URL. On mount we read
 * the hash (viewport) and the `r` search param (active region); on
 * `commit()` calls the URL is rewritten via `history.replaceState` so panning
 * the map does not flood the history stack.
 *
 * The hash format is owned by `lib/url-state.ts` and is not changed here.
 * The region key is serialised as `?r=europe-romania` in the query string so
 * it survives copy-paste through chat apps that rewrite hash fragments.
 */

/** Value captured from the URL at mount. */
export interface InitialUrlState {
  viewport: MapViewport;
  /** Canonical hyphenated region key, or null if absent/malformed. */
  region: string | null;
}

/** Values the caller wants to commit back to the URL. */
export interface UrlCommit {
  viewport: MapViewport;
  region: string | null;
}

export interface UseUrlViewportResult {
  initial: InitialUrlState | null;
  commit: (state: UrlCommit) => void;
}

const REGION_PARAM = "r";

function readRegionFromSearch(search: string): string | null {
  if (!search) return null;
  const params = new URLSearchParams(
    search.startsWith("?") ? search.slice(1) : search,
  );
  const raw = params.get(REGION_PARAM);
  if (raw && isCanonicalRegionKey(raw)) return raw;
  return null;
}

function buildNextSearch(search: string, region: string | null): string {
  const params = new URLSearchParams(
    search.startsWith("?") ? search.slice(1) : search,
  );
  if (region && isCanonicalRegionKey(region)) {
    params.set(REGION_PARAM, region);
  } else {
    params.delete(REGION_PARAM);
  }
  const qs = params.toString();
  return qs.length > 0 ? `?${qs}` : "";
}

/**
 * React hook that owns the URL <-> viewport/region round-trip. Map-library
 * agnostic; MapView reads `initial` for its starting state and calls
 * `commit` from the MapLibre `moveend` handler plus whenever `activeRegion`
 * changes in the store.
 */
export function useUrlViewport(
  fallbackViewport: MapViewport,
): UseUrlViewportResult {
  const [initial, setInitial] = useState<InitialUrlState | null>(null);
  const lastHash = useRef<string>("");
  const lastSearch = useRef<string>("");

  useEffect(() => {
    if (typeof window === "undefined") return;
    const parsedViewport = parseHash(window.location.hash) ?? fallbackViewport;
    const parsedRegion = readRegionFromSearch(window.location.search);
    setInitial({ viewport: parsedViewport, region: parsedRegion });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const commit = useCallback((state: UrlCommit) => {
    if (typeof window === "undefined") return;
    const hash = `#${formatHash(state.viewport)}`;
    const nextSearch = buildNextSearch(window.location.search, state.region);
    if (hash === lastHash.current && nextSearch === lastSearch.current) return;
    lastHash.current = hash;
    lastSearch.current = nextSearch;
    const { pathname } = window.location;
    window.history.replaceState(null, "", `${pathname}${nextSearch}${hash}`);
  }, []);

  return { initial, commit };
}
