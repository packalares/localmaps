"use client";

import { RefreshCw, Trash2 } from "lucide-react";
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
  pendingAction: "update" | "schedule" | "delete" | null;
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
  pendingAction,
}: InstalledCardProps) {
  const display = formatRegionState(region.state);

  return (
    <article
      aria-label={region.displayName}
      className="flex flex-col gap-2 border-b border-border px-3 py-3 last:border-0"
    >
      <header className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="truncate font-medium">{region.displayName}</div>
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
