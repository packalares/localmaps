"use client";

import { useCallback, useMemo } from "react";
import { Clock } from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { ResultCard } from "./ResultCard";
import {
  HISTORY_STORAGE_KEY,
  readHistory,
  useRecentHistory,
} from "@/lib/search/history";

/**
 * Recent search history.
 *
 * The gateway exposes no `/api/geocode/history` endpoint — see the
 * OpenAPI contract — so we persist to `localStorage` under a single key.
 * Each entry is a previously-picked `GeocodeResult`. Reads are
 * centralised in `lib/search/history.ts`; this component only owns
 * push/clear writes.
 */

const DEFAULT_MAX_ENTRIES = 10;

export interface RecentHistoryProps {
  /** Click handler — consumers replay the selection through the map. */
  onSelect: (result: GeocodeResult) => void;
  /** Reference point for the distance badge (current map centre). */
  origin?: { lat: number; lon: number } | null;
  /** Maximum entries kept; older entries drop off the tail. */
  maxEntries?: number;
}

function writeHistory(entries: GeocodeResult[]): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(entries));
  } catch {
    // Ignore quota / unavailable storage.
    return;
  }
  // Same-tab subscribers (useRecentHistory) listen for this so they can
  // refresh without polling. The native `storage` event does NOT fire
  // for the window that performed the write.
  try {
    window.dispatchEvent(new CustomEvent("localmaps.history.changed"));
  } catch {
    /* CustomEvent unavailable in some headless test envs — ignore. */
  }
}

/** Public helper so SearchPanel can push after every successful pick. */
export function pushHistoryEntry(
  result: GeocodeResult,
  maxEntries = DEFAULT_MAX_ENTRIES,
): void {
  const current = readHistory();
  const filtered = current.filter((r) => r.id !== result.id);
  const next = [result, ...filtered].slice(0, maxEntries);
  writeHistory(next);
}

/** Public helper to clear everything. */
export function clearHistory(): void {
  writeHistory([]);
}

export function RecentHistory({
  onSelect,
  origin,
  maxEntries = DEFAULT_MAX_ENTRIES,
}: RecentHistoryProps) {
  const history = useRecentHistory();
  const entries = useMemo(
    () => history.slice(0, maxEntries),
    [history, maxEntries],
  );

  const onClear = useCallback(() => {
    clearHistory();
  }, []);

  if (entries.length === 0) return null;

  return (
    <section className="flex flex-col" aria-label="Recent searches">
      <header className="flex items-center justify-between px-3 pb-1 pt-2">
        <div className="flex items-center gap-1.5 text-xs font-medium uppercase tracking-wide text-muted-foreground">
          <Clock className="h-3.5 w-3.5" aria-hidden="true" />
          <span>Recent</span>
        </div>
        <button
          type="button"
          onClick={onClear}
          className={cn(
            "inline-flex items-center gap-1 rounded px-1 py-0.5 text-xs text-muted-foreground",
            "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
          aria-label="Clear search history"
        >
          <span>Clear all</span>
        </button>
      </header>
      <div
        role="listbox"
        aria-label="Recent searches"
        className="flex flex-col gap-0.5 px-1 pb-1"
      >
        {entries.map((entry) => (
          <ResultCard
            key={entry.id}
            result={entry}
            origin={origin}
            onSelect={onSelect}
          />
        ))}
      </div>
    </section>
  );
}
