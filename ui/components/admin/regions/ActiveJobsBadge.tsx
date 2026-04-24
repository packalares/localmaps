"use client";

import { Loader2 } from "lucide-react";
import { cn } from "@/lib/utils";

/**
 * Compact pill shown at the top of the Regions page. Renders nothing
 * when there are zero active jobs, so the chrome is quiet by default.
 */
export interface ActiveJobsBadgeProps {
  count: number;
  className?: string;
}

export function ActiveJobsBadge({ count, className }: ActiveJobsBadgeProps) {
  if (count <= 0) return null;
  return (
    <span
      role="status"
      aria-live="polite"
      aria-label={`${count} active ${count === 1 ? "job" : "jobs"}`}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full bg-amber-100 px-2.5 py-1 text-xs font-medium text-amber-800 dark:bg-amber-900/40 dark:text-amber-200",
        className,
      )}
    >
      <Loader2 className="h-3 w-3 animate-spin" aria-hidden="true" />
      {count} active {count === 1 ? "job" : "jobs"}
    </span>
  );
}
