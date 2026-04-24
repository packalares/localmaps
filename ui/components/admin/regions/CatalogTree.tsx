"use client";

import { useMemo, useState } from "react";
import { Search as SearchIcon } from "lucide-react";
import type { Region, RegionCatalogEntry } from "@/lib/api/schemas";
import {
  filterCatalog,
  flattenCatalog,
} from "@/lib/admin/regions/use-regions-admin";
import { CatalogRow } from "./CatalogRow";
import { cn } from "@/lib/utils";

/**
 * Collapsible tree of the Geofabrik catalogue. Continents expand to
 * countries; countries may expand to sub-regions. A search box above
 * the tree narrows the visible set — matching ancestors stay so the
 * structure remains traversable.
 */
export interface CatalogTreeProps {
  entries: RegionCatalogEntry[];
  installedByName: Map<string, Region>;
  onInstall: (entry: RegionCatalogEntry) => void;
  className?: string;
}

export function CatalogTree({
  entries,
  installedByName,
  onInstall,
  className,
}: CatalogTreeProps) {
  const [query, setQuery] = useState("");
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());

  const filtered = useMemo(
    () => filterCatalog(entries, query),
    [entries, query],
  );

  // When the user is searching, auto-expand every matching ancestor so
  // descendants stay visible; when the search is cleared we keep the
  // user's manual open/closed state.
  const effectiveExpanded = useMemo(() => {
    if (!query.trim()) return expanded;
    const out = new Set(expanded);
    const walk = (nodes: RegionCatalogEntry[]) => {
      for (const n of nodes) {
        if (n.children && n.children.length > 0) {
          out.add(n.name);
          walk(n.children);
        }
      }
    };
    walk(filtered);
    return out;
  }, [expanded, filtered, query]);

  const flat = useMemo(
    () =>
      flattenCatalog(filtered).filter(({ entry, depth }) => {
        if (depth === 0) return true;
        // Walk up: if any ancestor is collapsed, hide this node.
        const segments = entry.name.split("/");
        for (let i = 1; i < segments.length; i++) {
          const ancestor = segments.slice(0, i).join("/");
          if (!effectiveExpanded.has(ancestor)) return false;
        }
        return true;
      }),
    [filtered, effectiveExpanded],
  );

  const toggle = (name: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });
  };

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
        {flat.length === 0 ? (
          <p className="p-6 text-center text-sm text-muted-foreground">
            {query.trim()
              ? "No regions match your search."
              : "Catalogue is empty."}
          </p>
        ) : (
          <div className="flex flex-col py-1">
            {flat.map(({ entry, depth }) => {
              const hasChildren =
                !!entry.children && entry.children.length > 0;
              return (
                <CatalogRow
                  key={entry.name}
                  entry={entry}
                  depth={depth}
                  expanded={effectiveExpanded.has(entry.name)}
                  onToggle={toggle}
                  hasChildren={hasChildren}
                  installed={installedByName.get(entry.name) ?? null}
                  onInstall={onInstall}
                />
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
