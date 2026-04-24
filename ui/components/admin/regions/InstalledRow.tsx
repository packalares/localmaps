"use client";

import { RefreshCw, Trash2 } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import type { Region } from "@/lib/api/schemas";
import { formatBytes } from "@/lib/admin/regions/format-bytes";
import {
  formatNextUpdate,
} from "@/lib/admin/regions/format-schedule";
import { formatRegionState } from "@/lib/admin/regions/format-state";
import { cn } from "@/lib/utils";
import { ProgressCell } from "./ProgressCell";
import { ScheduleDropdown } from "./ScheduleDropdown";

/**
 * One row in the installed-regions table. Columns mirror the spec:
 * region, state chip, size, last-updated, next-update, schedule, and
 * actions. Actions depend on the region state — ready regions get
 * Update+Schedule+Delete; failed regions get Retry+Delete; in-flight
 * regions have their progress shown in the state column and no
 * destructive actions.
 */
export interface InstalledRowProps {
  region: Region;
  onUpdateNow: (region: Region) => void;
  onScheduleChange: (region: Region, next: string) => void;
  onDelete: (region: Region) => void;
  pendingAction: "update" | "schedule" | "delete" | null;
}

function formatMaybeDate(
  iso: string | null | undefined,
): string {
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

export function InstalledRow({
  region,
  onUpdateNow,
  onScheduleChange,
  onDelete,
  pendingAction,
}: InstalledRowProps) {
  const display = formatRegionState(region.state);

  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-3 py-3 align-top">
        <div className="flex flex-col">
          <span className="font-medium">{region.displayName}</span>
          <span className="font-mono text-xs text-muted-foreground">
            {region.name}
          </span>
        </div>
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex flex-col gap-2">
          <span
            className={cn(
              "inline-flex w-fit items-center rounded-full px-2 py-0.5 text-xs font-medium",
              display.chipClass,
            )}
          >
            {display.label}
          </span>
          {display.inProgress ? <ProgressCell region={region} /> : null}
          {display.isFailed && region.lastError ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="max-w-[24ch] cursor-help truncate text-xs text-muted-foreground">
                  {region.lastError}
                </span>
              </TooltipTrigger>
              <TooltipContent side="bottom" align="start" className="max-w-sm">
                <pre className="whitespace-pre-wrap text-xs">
                  {region.lastError}
                </pre>
              </TooltipContent>
            </Tooltip>
          ) : null}
        </div>
      </td>
      <td className="px-3 py-3 align-top tabular-nums text-sm">
        {formatBytes(region.diskBytes ?? null)}
      </td>
      <td className="px-3 py-3 align-top text-sm">
        {formatMaybeDate(region.lastUpdatedAt)}
      </td>
      <td className="px-3 py-3 align-top text-sm">
        {formatNextUpdate(region.nextUpdateAt)}
      </td>
      <td className="px-3 py-3 align-top">
        <ScheduleDropdown
          value={region.schedule}
          onChange={(next) => onScheduleChange(region, next)}
          disabled={pendingAction === "schedule"}
        />
      </td>
      <td className="px-3 py-3 align-top">
        <div className="flex items-center justify-end gap-1">
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
              <RefreshCw className="mr-1.5 h-3.5 w-3.5" aria-hidden="true" />
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
      </td>
    </tr>
  );
}
