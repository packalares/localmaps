"use client";

/**
 * Centralised reader for the recent-search localStorage history. Three
 * UI surfaces previously rolled their own validators (RecentHistory,
 * LeftIconRail, DirectionsPanel) — they now all flow through here so a
 * single zod-checked + storage-event-aware path exists.
 *
 * Writes still live in `components/search/RecentHistory.tsx` because the
 * push semantics (filter-by-id + LRU clamp) are tied to the SearchPanel
 * selection flow. Once Saved-list lands we can collapse those into here.
 */

import { useEffect, useState } from "react";
import {
  GeocodeResultSchema,
  type GeocodeResult,
} from "@/lib/api/schemas";

/** Single source of truth for the localStorage key. */
export const HISTORY_STORAGE_KEY = "localmaps.search.history.v1";

/**
 * Pure helper used by every consumer: read + zod-validate + dedupe by
 * id. Returns an empty array when storage is unavailable, the payload
 * is corrupt, or the value is missing.
 */
export function readHistory(): GeocodeResult[] {
  if (typeof window === "undefined") return [];
  let raw: string | null = null;
  try {
    raw = window.localStorage.getItem(HISTORY_STORAGE_KEY);
  } catch {
    return [];
  }
  if (!raw) return [];
  let parsed: unknown;
  try {
    parsed = JSON.parse(raw);
  } catch {
    return [];
  }
  if (!Array.isArray(parsed)) return [];
  const out: GeocodeResult[] = [];
  const seen = new Set<string>();
  for (const item of parsed) {
    const safe = GeocodeResultSchema.safeParse(item);
    if (!safe.success) continue;
    if (seen.has(safe.data.id)) continue;
    seen.add(safe.data.id);
    out.push(safe.data);
  }
  return out;
}

/**
 * React hook: returns the current history, refreshes on cross-tab
 * `storage` events, and supports an optional polling interval for the
 * one same-tab edge-case where the writer is in the same window
 * (`storage` events do NOT fire for same-window writes). The push
 * helper (`pushHistoryEntry`) dispatches a synthetic `storage`-shape
 * event so the same hook instance refreshes after a write.
 */
export function useRecentHistory(): GeocodeResult[] {
  // Start empty so SSR matches client's first paint (Next.js hydration).
  // The useEffect below populates the real value after mount.
  const [entries, setEntries] = useState<GeocodeResult[]>([]);

  useEffect(() => {
    const refresh = () => setEntries(readHistory());
    refresh();
    const onStorage = (event: StorageEvent) => {
      if (event.key === HISTORY_STORAGE_KEY) refresh();
    };
    const onLocal = () => refresh();
    window.addEventListener("storage", onStorage);
    // Same-tab listener: the writer (RecentHistory.tsx) dispatches a
    // CustomEvent on every push so subscribers in the same window can
    // refresh without polling.
    window.addEventListener("localmaps.history.changed", onLocal);
    return () => {
      window.removeEventListener("storage", onStorage);
      window.removeEventListener("localmaps.history.changed", onLocal);
    };
  }, []);

  return entries;
}
