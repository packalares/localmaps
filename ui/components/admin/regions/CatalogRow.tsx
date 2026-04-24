"use client";

import { ChevronDown, ChevronRight, Check, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { RegionCatalogEntry, Region } from "@/lib/api/schemas";
import { formatBytes } from "@/lib/admin/regions/format-bytes";
import { formatRegionState } from "@/lib/admin/regions/format-state";
import { cn } from "@/lib/utils";

/**
 * One row in the catalogue tree. Presents a disclosure chevron for
 * entries with children plus an install action on leaves (or any node
 * that can itself be installed — continents and sub-regions are valid
 * targets per openapi `RegionCatalogEntry`).
 */
export interface CatalogRowProps {
  entry: RegionCatalogEntry;
  depth: number;
  expanded: boolean;
  onToggle: (name: string) => void;
  hasChildren: boolean;
  installed?: Region | null;
  onInstall: (entry: RegionCatalogEntry) => void;
}

export function CatalogRow({
  entry,
  depth,
  expanded,
  onToggle,
  hasChildren,
  installed,
  onInstall,
}: CatalogRowProps) {
  const state = installed ? formatRegionState(installed.state) : null;
  const isInstalled = !!installed && installed.state !== "archived";

  return (
    <div
      role="treeitem"
      aria-expanded={hasChildren ? expanded : undefined}
      aria-level={depth + 1}
      aria-selected={false}
      aria-label={entry.displayName}
      className={cn(
        "group flex items-center gap-2 rounded-md px-2 py-1.5 text-sm",
        "hover:bg-muted/60",
      )}
      style={{ paddingLeft: `${depth * 16 + 8}px` }}
    >
      <button
        type="button"
        aria-label={hasChildren ? (expanded ? "Collapse" : "Expand") : undefined}
        className={cn(
          "flex h-6 w-6 shrink-0 items-center justify-center rounded-sm",
          hasChildren ? "hover:bg-muted" : "opacity-0",
        )}
        onClick={() => hasChildren && onToggle(entry.name)}
        tabIndex={hasChildren ? 0 : -1}
        disabled={!hasChildren}
      >
        {hasChildren ? (
          expanded ? (
            <ChevronDown className="h-4 w-4" aria-hidden="true" />
          ) : (
            <ChevronRight className="h-4 w-4" aria-hidden="true" />
          )
        ) : null}
      </button>

      <span className="flex-1 truncate">
        <span className="font-medium">{entry.displayName}</span>
        {entry.sourcePbfBytes ? (
          <span className="ml-2 text-xs text-muted-foreground">
            {formatBytes(entry.sourcePbfBytes)}
          </span>
        ) : null}
      </span>

      {isInstalled && state ? (
        <span
          className={cn(
            "inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs",
            state.chipClass,
          )}
          aria-label={`Already ${state.label.toLowerCase()}`}
        >
          <Check className="h-3 w-3" aria-hidden="true" />
          {state.label}
        </span>
      ) : (
        <Button
          size="sm"
          variant="outline"
          className="h-7 gap-1 opacity-0 transition-opacity group-hover:opacity-100 focus:opacity-100"
          onClick={() => onInstall(entry)}
          aria-label={`Install ${entry.displayName}`}
        >
          <Download className="h-3 w-3" aria-hidden="true" />
          Install
        </Button>
      )}
    </div>
  );
}
