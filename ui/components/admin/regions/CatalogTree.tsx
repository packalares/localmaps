"use client";

import { ChevronDown, ChevronRight, Search as SearchIcon } from "lucide-react";
import { useMemo, useState } from "react";
import type { Region, RegionCatalogEntry } from "@/lib/api/schemas";
import { filterCatalog } from "@/lib/admin/regions/use-regions-admin";
import { CatalogRow } from "./CatalogRow";
import { cn } from "@/lib/utils";

/**
 * Collapsible, continent-grouped view of the Geofabrik catalogue.
 *
 * The wire format is flexible: the gateway may return a nested tree
 * (continent → country → subregion) or a flat list where every entry
 * carries a `parent` field. This component handles both by flattening
 * everything first, then regrouping by the entry's ultimate continent
 * (walked via the `parent` chain). Each continent renders as a single
 * collapsible header — countries and subregions only appear once the
 * user expands the continent.
 *
 * A search box above the tree filters the visible set. Any continent
 * containing a match is auto-expanded for the duration of the search.
 */
export interface CatalogTreeProps {
  entries: RegionCatalogEntry[];
  installedByName: Map<string, Region>;
  onInstall: (entry: RegionCatalogEntry) => void;
  className?: string;
}

interface FlatNode {
  entry: RegionCatalogEntry;
  /** The canonical name of the continent this entry belongs to. */
  continent: string;
  /** Depth relative to the continent (0 for the continent header, 1+ below). */
  depth: number;
}

/** Walk `entries` (tree or flat) and emit one FlatNode per entry. */
function flattenWithContinent(
  entries: RegionCatalogEntry[],
): { byName: Map<string, RegionCatalogEntry>; all: RegionCatalogEntry[] } {
  const byName = new Map<string, RegionCatalogEntry>();
  const all: RegionCatalogEntry[] = [];
  const walk = (nodes: RegionCatalogEntry[]) => {
    for (const n of nodes) {
      byName.set(n.name, n);
      all.push(n);
      if (n.children && n.children.length > 0) walk(n.children);
    }
  };
  walk(entries);
  return { byName, all };
}

/** Resolve the top-level (continent) name by walking up the parent chain. */
function resolveContinent(
  entry: RegionCatalogEntry,
  byName: Map<string, RegionCatalogEntry>,
): string {
  let cursor: RegionCatalogEntry | undefined = entry;
  const seen = new Set<string>();
  while (cursor?.parent) {
    if (seen.has(cursor.name)) break; // cycle guard
    seen.add(cursor.name);
    const next = byName.get(cursor.parent);
    if (!next) return cursor.parent; // orphan — treat parent as the continent
    cursor = next;
  }
  return cursor?.name ?? entry.name;
}

/** Depth of an entry within its continent (0 = continent row itself). */
function continentDepth(
  entry: RegionCatalogEntry,
  byName: Map<string, RegionCatalogEntry>,
): number {
  let depth = 0;
  let cursor: RegionCatalogEntry | undefined = entry;
  const seen = new Set<string>();
  while (cursor?.parent) {
    if (seen.has(cursor.name)) break;
    seen.add(cursor.name);
    depth += 1;
    const next = byName.get(cursor.parent);
    if (!next) break;
    cursor = next;
  }
  return depth;
}

/** Humanise a Geofabrik continent key (e.g. `australia-oceania`). */
function prettyContinentName(
  key: string,
  byName: Map<string, RegionCatalogEntry>,
): string {
  const node = byName.get(key);
  if (node?.displayName) return node.displayName;
  return key
    .split(/[-_]/g)
    .map((s) => (s.length === 0 ? s : s[0]!.toUpperCase() + s.slice(1)))
    .join(" ");
}

export function CatalogTree({
  entries,
  installedByName,
  onInstall,
  className,
}: CatalogTreeProps) {
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());

  // Filter first so we can build a smaller continent map from the
  // reduced set. `filterCatalog` already handles both tree + flat by
  // walking `children` where they exist.
  const filtered = useMemo(
    () => filterCatalog(entries, query),
    [entries, query],
  );

  const { flatNodes, continentKeys } = useMemo(() => {
    const { byName, all } = flattenWithContinent(filtered);
    const nodes: FlatNode[] = all.map((entry) => ({
      entry,
      continent: resolveContinent(entry, byName),
      depth: continentDepth(entry, byName),
    }));
    // Unique list of continents in the filtered set, sorted by their
    // pretty name so admins see Africa → Antarctica → Asia → … in the
    // expected order.
    const seen = new Set<string>();
    const keys: string[] = [];
    for (const n of nodes) {
      if (!seen.has(n.continent)) {
        seen.add(n.continent);
        keys.push(n.continent);
      }
    }
    keys.sort((a, b) =>
      prettyContinentName(a, byName).localeCompare(
        prettyContinentName(b, byName),
      ),
    );
    // Sort children alphabetically by displayName within each continent.
    nodes.sort((a, b) => {
      if (a.continent !== b.continent) return 0; // preserve continent grouping
      if (a.depth === 0) return -1;
      if (b.depth === 0) return 1;
      return a.entry.displayName.localeCompare(b.entry.displayName);
    });
    return { flatNodes: nodes, continentKeys: keys };
  }, [filtered]);

  // When the user types a query, auto-expand every continent that has
  // a match; when the query is cleared we respect the user's manual
  // open/closed state.
  const effectiveExpanded = useMemo(() => {
    if (!query.trim()) return expanded;
    const out = new Set(expanded);
    for (const key of continentKeys) out.add(key);
    return out;
  }, [expanded, continentKeys, query]);

  const toggle = (name: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

  // Bucket the flattened nodes by continent so the render loop is a
  // simple `for (key of continentKeys) renderBucket(buckets[key])`.
  const buckets = useMemo(() => {
    const m = new Map<string, FlatNode[]>();
    for (const key of continentKeys) m.set(key, []);
    for (const n of flatNodes) {
      const bucket = m.get(n.continent);
      if (bucket) bucket.push(n);
    }
    return m;
  }, [continentKeys, flatNodes]);

  const hasAnyEntries = flatNodes.length > 0;

  return (
    <div className={cn("flex min-h-0 flex-col", className)}>
      <div className="relative p-2">
        <SearchIcon
          className="pointer-events-none absolute left-4 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <input
          type="search"
          placeholder="Search continents and countries…"
          aria-label="Filter catalogue"
          value={query}
          onChange={(ev) => setQuery(ev.target.value)}
          className={cn(
            "h-9 w-full rounded-md border border-input bg-background pl-8 pr-3 text-sm",
            "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        />
      </div>

      <div
        role="tree"
        aria-label="Geofabrik catalogue"
        className="min-h-0 flex-1 overflow-y-auto border-t border-border"
      >
        {!hasAnyEntries ? (
          <p className="p-6 text-center text-sm text-muted-foreground">
            {query.trim()
              ? "No regions match your search."
              : "Catalogue is empty."}
          </p>
        ) : (
          <div className="flex flex-col py-1">
            {continentKeys.map((key) => {
              const bucket = buckets.get(key) ?? [];
              const header = bucket.find((n) => n.depth === 0);
              const childCount = bucket.filter((n) => n.depth > 0).length;
              const isExpanded = effectiveExpanded.has(key);
              return (
                <ContinentGroup
                  key={key}
                  id={key}
                  label={
                    header?.entry.displayName ??
                    // Fall back to a humanised key when the API omits a
                    // continent row (orphan children with `parent` set).
                    key
                      .split(/[-_]/g)
                      .map((s) =>
                        s.length === 0 ? s : s[0]!.toUpperCase() + s.slice(1),
                      )
                      .join(" ")
                  }
                  childCount={childCount}
                  expanded={isExpanded}
                  onToggle={() => toggle(key)}
                  header={header}
                  installedByName={installedByName}
                  onInstall={onInstall}
                  rows={bucket.filter((n) => n.depth > 0)}
                />
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}

interface ContinentGroupProps {
  id: string;
  label: string;
  childCount: number;
  expanded: boolean;
  onToggle: () => void;
  header: FlatNode | undefined;
  installedByName: Map<string, Region>;
  onInstall: (entry: RegionCatalogEntry) => void;
  rows: FlatNode[];
}

function ContinentGroup({
  id,
  label,
  childCount,
  expanded,
  onToggle,
  header,
  installedByName,
  onInstall,
  rows,
}: ContinentGroupProps) {
  const installed = header ? installedByName.get(header.entry.name) : undefined;
  return (
    <div className="flex flex-col">
      <div
        role="treeitem"
        aria-expanded={expanded}
        aria-level={1}
        aria-label={label}
        aria-selected={false}
        aria-controls={`catalog-group-${id}`}
        className={cn(
          "group flex items-center gap-2 rounded-md px-2 py-2 text-sm font-medium",
          "hover:bg-muted/60",
        )}
      >
        <button
          type="button"
          onClick={onToggle}
          aria-label={expanded ? "Collapse" : "Expand"}
          className={cn(
            "flex h-6 w-6 shrink-0 items-center justify-center rounded-sm",
            "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          {expanded ? (
            <ChevronDown className="h-4 w-4" aria-hidden="true" />
          ) : (
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          )}
        </button>
        <button
          type="button"
          onClick={onToggle}
          className="flex flex-1 items-center gap-2 truncate text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <span className="flex-1 truncate">{label}</span>
          {childCount > 0 ? (
            <span className="text-xs text-muted-foreground" aria-hidden="true">
              {childCount}
            </span>
          ) : null}
        </button>
        {header && !installed ? (
          <button
            type="button"
            onClick={(ev) => {
              ev.stopPropagation();
              onInstall(header.entry);
            }}
            aria-label={`Install ${header.entry.displayName}`}
            className={cn(
              "h-7 rounded-md border border-input px-2 text-xs opacity-0",
              "transition-opacity hover:bg-accent hover:text-accent-foreground",
              "group-hover:opacity-100 focus:opacity-100",
              "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            )}
          >
            Install
          </button>
        ) : null}
      </div>
      {expanded && rows.length > 0 ? (
        <div
          id={`catalog-group-${id}`}
          role="group"
          aria-label={`${label} regions`}
          className="flex flex-col"
        >
          {rows.map((n) => (
            <CatalogRow
              key={n.entry.name}
              entry={n.entry}
              depth={n.depth}
              expanded={false}
              onToggle={() => {}}
              hasChildren={false}
              installed={installedByName.get(n.entry.name) ?? null}
              onInstall={onInstall}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}
