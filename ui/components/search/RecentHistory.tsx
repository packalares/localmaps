"use client";

import { useCallback, useEffect, useState } from "react";
import { Clock, X } from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";
import { GeocodeResultSchema } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { ResultCard } from "./ResultCard";

/**
 * Recent search history.
 *
 * The gateway exposes no `/api/geocode/history` endpoint — see the
 * OpenAPI contract — so we persist to `localStorage` under a single key.
 * Each entry is a previously-picked `GeocodeResult`. Validated via zod
 * on read so stale payloads from earlier UI versions don't poison
 * render.
 */

const STORAGE_KEY = "localmaps.search.history.v1";
const DEFAULT_MAX_ENTRIES = 10;

export interface RecentHistoryProps {
  /** Click handler — consumers replay the selection through the map. */
  onSelect: (result: GeocodeResult) => void;
  /** Reference point for the distance badge (current map centre). */
  origin?: { lat: number; lon: number } | null;
  /** Maximum entries kept; older entries drop off the tail. */
  maxEntries?: number;
}

/** Imperative accessors shared between the hook and the component. */
function readHistory(): GeocodeResult[] {
  if (typeof window === "undefined") return [];
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(STORAGE_KEY);
  } catch {
    return [];
  }
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    const out: GeocodeResult[] = [];
    for (const item of parsed) {
      const safe = GeocodeResultSchema.safeParse(item);
      if (safe.success) out.push(safe.data);
    }
    return out;
  } catch {
    return [];
  }
}

function writeHistory(entries: GeocodeResult[]): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(entries));
  } catch {
    // Ignore quota / unavailable storage.
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
  const [entries, setEntries] = useState<GeocodeResult[]>([]);

  useEffect(() => {
    setEntries(readHistory().slice(0, maxEntries));
    const onStorage = (event: StorageEvent) => {
      if (event.key === STORAGE_KEY) {
        setEntries(readHistory().slice(0, maxEntries));
      }
    };
    window.addEventListener("storage", onStorage);
    return () => window.removeEventListener("storage", onStorage);
  }, [maxEntries]);

  const onClear = useCallback(() => {
    clearHistory();
    setEntries([]);
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
          <X className="h-3 w-3" aria-hidden="true" />
          <span>Clear</span>
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
