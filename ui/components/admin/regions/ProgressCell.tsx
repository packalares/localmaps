"use client";

import { useJob } from "@/lib/api/hooks";
import type { Region } from "@/lib/api/schemas";
import { formatRegionState } from "@/lib/admin/regions/format-state";
import { cn } from "@/lib/utils";

/**
 * Per-row progress bar driven by the region's `activeJobId`. When no
 * job is attached (ready / archived / failed with no retry in flight),
 * renders the stage text alone so the table row height stays stable.
 *
 * Falls back to polling (refetchInterval = 3s) when the WS stream is
 * unavailable; the WS hook, if present in the page, populates the same
 * cache key and short-circuits the HTTP poll.
 */
export interface ProgressCellProps {
  region: Region;
}

export function ProgressCell({ region }: ProgressCellProps) {
  const display = formatRegionState(region.state);
  const jobQ = useJob(region.activeJobId ?? null, {
    enabled: display.inProgress && !!region.activeJobId,
    refetchIntervalMs: display.inProgress ? 3_000 : undefined,
  });

  const progress = jobQ.data?.progress ?? null;
  const stage = jobQ.data?.message || region.stateDetail || display.stage;
  const pct =
    typeof progress === "number" && Number.isFinite(progress)
      ? Math.max(0, Math.min(100, Math.round(progress * 100)))
      : null;

  if (!display.inProgress) {
    return (
      <div className="text-sm text-muted-foreground" aria-hidden="true">
        {display.stage}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-1" role="group" aria-label="Install progress">
      <div className="flex items-center justify-between text-xs">
        <span className="truncate text-foreground">{stage}</span>
        <span className="tabular-nums text-muted-foreground">
          {pct !== null ? `${pct}%` : "…"}
        </span>
      </div>
      <div
        role="progressbar"
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={pct ?? undefined}
        aria-label={`${stage}${pct !== null ? ` ${pct}%` : ""}`}
        className="h-1.5 w-full overflow-hidden rounded-full bg-muted"
      >
        <div
          className={cn(
            "h-full bg-amber-500 transition-[width] duration-300 dark:bg-amber-400",
            pct === null && "animate-pulse w-1/3",
          )}
          style={pct !== null ? { width: `${pct}%` } : undefined}
        />
      </div>
    </div>
  );
}
