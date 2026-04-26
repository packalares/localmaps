"use client";

import { Navigation, RefreshCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import type { Region } from "@/lib/api/schemas";
import { formatBytes } from "@/lib/admin/regions/format-bytes";
import { formatNextUpdate } from "@/lib/admin/regions/format-schedule";
import { formatRegionState } from "@/lib/admin/regions/format-state";
import { cn } from "@/lib/utils";
import { ProgressCell } from "./ProgressCell";
import { ScheduleDropdown } from "./ScheduleDropdown";

/**
 * Mobile-layout sibling to `InstalledRow`. Rendered below 768px when
 * horizontal scrolling a 7-column table is unusable. Displays the same
 * data as a stacked card: region name + state chip on top, key/value
 * rows in the middle, action buttons at the bottom.
 */
export interface InstalledCardProps {
  region: Region;
  onUpdateNow: (region: Region) => void;
  onScheduleChange: (region: Region, next: string) => void;
  onDelete: (region: Region) => void;
  /** Optional: when provided, render a "Use for routing" action. */
  onActivate?: (region: Region) => void;
  /** True when this region is the active routing target. */
  isActiveRouting?: boolean;
  pendingAction: "update" | "schedule" | "delete" | "activate" | null;
}

function formatMaybeDate(iso: string | null | undefined): string {
  if (!iso) return "—";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "—";
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

export function InstalledCard({
  region,
  onUpdateNow,
  onScheduleChange,
  onDelete,
  onActivate,
  isActiveRouting,
  pendingAction,
}: InstalledCardProps) {
  const display = formatRegionState(region.state);
  const canActivate = onActivate && display.isReady && !isActiveRouting;

  return (
    <article
      aria-label={region.displayName}
      data-active-routing={isActiveRouting ? "true" : undefined}
      className={cn(
        "flex flex-col gap-2 border-b border-border px-3 py-3 last:border-0",
        isActiveRouting ? "bg-primary/5" : undefined,
      )}
    >
      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 truncate font-medium">
            <span className="truncate">{region.displayName}</span>
            {isActiveRouting ? (
              <span
                aria-label="Active routing region"
                className="inline-flex shrink-0 items-center rounded-full bg-primary/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-primary"
              >
                Active
              </span>
            ) : null}
          </div>
          <div className="truncate font-mono text-xs text-muted-foreground">
            {region.name}
          </div>
        </div>
        <span
          className={cn(
            "shrink-0 rounded-full px-2 py-0.5 text-xs font-medium",
            display.chipClass,
          )}
        >
          {display.label}
        </span>
      </header>

      {display.inProgress ? <ProgressCell region={region} /> : null}
      {display.isFailed && region.lastError ? (
        <p className="truncate text-xs text-muted-foreground" title={region.lastError}>
          {region.lastError}
        </p>
      ) : null}

      <dl className="grid grid-cols-2 gap-x-3 gap-y-1 text-xs">
        <dt className="text-muted-foreground">Size</dt>
        <dd className="tabular-nums">{formatBytes(region.diskBytes ?? null)}</dd>
        <dt className="text-muted-foreground">Last updated</dt>
        <dd>{formatMaybeDate(region.lastUpdatedAt)}</dd>
        <dt className="text-muted-foreground">Next update</dt>
        <dd>{formatNextUpdate(region.nextUpdateAt)}</dd>
      </dl>

      <div className="flex items-center justify-between gap-2 pt-1">
        <ScheduleDropdown
          value={region.schedule}
          onChange={(next) => onScheduleChange(region, next)}
          disabled={pendingAction === "schedule"}
        />
        <div className="flex items-center gap-1">
          {canActivate ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onActivate?.(region)}
              disabled={pendingAction === "activate"}
              aria-label={`Use ${region.displayName} for routing`}
            >
              <Navigation className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
              Use for routing
            </Button>
          ) : null}
          {display.isReady || display.isFailed ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => onUpdateNow(region)}
              disabled={pendingAction === "update" || display.inProgress}
              aria-label={
                display.isFailed
                  ? `Retry ${region.displayName}`
                  : `Update ${region.displayName} now`
              }
            >
              <RefreshCw className="mr-1 h-3.5 w-3.5" aria-hidden="true" />
              {display.isFailed ? "Retry" : "Update"}
            </Button>
          ) : null}
          <Button
            size="sm"
            variant="ghost"
            onClick={() => onDelete(region)}
            disabled={pendingAction === "delete"}
            aria-label={`Delete ${region.displayName}`}
          >
            <Trash2 className="h-3.5 w-3.5 text-destructive" aria-hidden="true" />
          </Button>
        </div>
      </div>
    </article>
  );
}
