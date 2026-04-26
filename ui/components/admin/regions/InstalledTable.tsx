"use client";

import type { Region } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";
import { useBreakpoint } from "@/lib/responsive/use-breakpoint";
import { InstalledRow } from "./InstalledRow";
import { InstalledCard } from "./InstalledCard";
import { useMessages } from "@/lib/i18n/provider";

/**
 * Table of installed + in-progress regions. Mirrors the Google-Material
 * admin pattern: sticky header, zebra hover, truncated region key.
 * The schedule column pins to the right on wide screens so the action
 * cluster is always anchored to the edge.
 *
 * Below 768px the table degrades into a stacked list of `InstalledCard`s
 * so the action buttons stay reachable without horizontal scroll.
 */
export interface InstalledTableProps {
  regions: Region[];
  pendingByName: Record<string, "update" | "schedule" | "delete" | "activate" | null>;
  onUpdateNow: (region: Region) => void;
  onScheduleChange: (region: Region, next: string) => void;
  onDelete: (region: Region) => void;
  /** Optional: when provided, render the "Use for routing" action. */
  onActivate?: (region: Region) => void;
  /** Canonical key of the region currently serving routing requests. */
  activeRegionName?: string | null;
  className?: string;
  /** Test seam: force the card layout regardless of viewport. */
  forceMobile?: boolean;
}

export function InstalledTable({
  regions,
  pendingByName,
  onUpdateNow,
  onScheduleChange,
  onDelete,
  onActivate,
  activeRegionName,
  className,
  forceMobile,
}: InstalledTableProps) {
  const bp = useBreakpoint();
  const isMobile = forceMobile ?? bp === "mobile";
  const { t } = useMessages();

  if (regions.length === 0) {
    return (
      <div
        className={cn(
          "flex min-h-0 flex-1 items-center justify-center p-12 text-center text-sm text-muted-foreground",
          className,
        )}
      >
        <div>
          <p className="mb-1 text-base font-medium text-foreground">
            {t("regions.empty.title")}
          </p>
          <p>{t("regions.empty.body")}</p>
        </div>
      </div>
    );
  }

  if (isMobile) {
    return (
      <div
        role="list"
        aria-label="Installed regions"
        className={cn("min-h-0 flex-1 overflow-auto", className)}
      >
        {regions.map((r) => (
          <InstalledCard
            key={r.name}
            region={r}
            onUpdateNow={onUpdateNow}
            onScheduleChange={onScheduleChange}
            onDelete={onDelete}
            onActivate={onActivate}
            isActiveRouting={activeRegionName === r.name}
            pendingAction={pendingByName[r.name] ?? null}
          />
        ))}
      </div>
    );
  }

  return (
    <div className={cn("min-h-0 flex-1 overflow-auto", className)}>
      <table className="w-full border-collapse text-left text-sm">
        <thead className="sticky top-0 z-10 bg-background/95 backdrop-blur">
          <tr className="border-b border-border text-xs uppercase tracking-wide text-muted-foreground">
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.region")}
            </th>
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.state")}
            </th>
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.size")}
            </th>
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.lastUpdated")}
            </th>
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.nextUpdate")}
            </th>
            <th scope="col" className="px-3 py-2 font-medium">
              {t("regions.table.schedule")}
            </th>
            <th scope="col" className="px-3 py-2 text-right font-medium">
              {t("regions.table.actions")}
            </th>
          </tr>
        </thead>
        <tbody>
          {regions.map((r) => (
            <InstalledRow
              key={r.name}
              region={r}
              onUpdateNow={onUpdateNow}
              onScheduleChange={onScheduleChange}
              onDelete={onDelete}
              onActivate={onActivate}
              isActiveRouting={activeRegionName === r.name}
              pendingAction={pendingByName[r.name] ?? null}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
