"use client";

import { useMemo } from "react";
import { useRegionCatalog, useRegions } from "@/lib/api/hooks";
import type {
  Region,
  RegionCatalogEntry,
  RegionsListResponse,
} from "@/lib/api/schemas";
import { formatRegionState } from "./format-state";

/**
 * Aggregator for the admin Regions page. Combines the installed-regions
 * query with the Geofabrik catalogue, keyed so the catalogue tree can
 * mark already-installed leaves without iterating the installed list
 * per row.
 */

export interface AdminRegionsData {
  regions: Region[];
  catalog: RegionCatalogEntry[];
  /** Map from Geofabrik name (`europe/romania`) → installed Region. */
  installedByName: Map<string, Region>;
  /** Count of regions currently in a long-running pipeline. */
  activeCount: number;
  isLoading: boolean;
  error: unknown;
  /** Timestamp for the "Catalogue refreshed" footer. */
  fetchedAt?: string;
}

export function useRegionsAdmin(): AdminRegionsData {
  const regionsQ = useRegions();
  const catalogQ = useRegionCatalog();

  const regions = useMemo(
    () => regionsQ.data?.regions ?? [],
    [regionsQ.data],
  );
  const catalog = useMemo(
    () => catalogQ.data?.catalog ?? [],
    [catalogQ.data],
  );

  const installedByName = useMemo(() => {
    const m = new Map<string, Region>();
    for (const r of regions) m.set(r.name, r);
    return m;
  }, [regions]);

  const activeCount = useMemo(
    () => regions.filter((r) => formatRegionState(r.state).inProgress).length,
    [regions],
  );

  return {
    regions,
    catalog,
    installedByName,
    activeCount,
    isLoading: regionsQ.isLoading || catalogQ.isLoading,
    error: regionsQ.error ?? catalogQ.error,
    fetchedAt: catalogQ.data?.fetchedAt,
  };
}

/** Flatten a catalogue tree into an ordered list with a depth hint. */
export function flattenCatalog(
  entries: RegionCatalogEntry[],
  depth = 0,
): Array<{ entry: RegionCatalogEntry; depth: number }> {
  const out: Array<{ entry: RegionCatalogEntry; depth: number }> = [];
  for (const entry of entries) {
    out.push({ entry, depth });
    if (entry.children && entry.children.length > 0) {
      out.push(...flattenCatalog(entry.children, depth + 1));
    }
  }
  return out;
}

/**
 * Filter a catalogue tree by a free-text query. Matches case-insensitively
 * against displayName or name; an ancestor is kept if any of its descendants
 * match, so the tree stays traversable.
 */
export function filterCatalog(
  entries: RegionCatalogEntry[],
  query: string,
): RegionCatalogEntry[] {
  const q = query.trim().toLowerCase();
  if (!q) return entries;
  const walk = (nodes: RegionCatalogEntry[]): RegionCatalogEntry[] => {
    const out: RegionCatalogEntry[] = [];
    for (const node of nodes) {
      const selfMatch =
        node.displayName.toLowerCase().includes(q) ||
        node.name.toLowerCase().includes(q);
      const kids = node.children ? walk(node.children) : [];
      if (selfMatch || kids.length > 0) {
        out.push({ ...node, children: kids.length > 0 ? kids : node.children });
      }
    }
    return out;
  };
  return walk(entries);
}

/** Helper used by tests. Not part of the hook surface but exported for reuse. */
export function buildAdminData(
  regionsResp: RegionsListResponse | undefined,
  catalogEntries: RegionCatalogEntry[] | undefined,
  fetchedAt: string | undefined,
): AdminRegionsData {
  const regions = regionsResp?.regions ?? [];
  const catalog = catalogEntries ?? [];
  const installedByName = new Map<string, Region>();
  for (const r of regions) installedByName.set(r.name, r);
  return {
    regions,
    catalog,
    installedByName,
    activeCount: regions.filter((r) => formatRegionState(r.state).inProgress)
      .length,
    isLoading: false,
    error: null,
    fetchedAt,
  };
}
