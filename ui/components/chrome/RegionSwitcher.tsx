"use client";

import { ChevronDown, Check, Globe } from "lucide-react";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";
import { useMapStore } from "@/lib/state/map";
import { toCanonicalRegionKey } from "@/lib/map/region-key";
import type { Region } from "@/lib/api/schemas";

/**
 * Tiny dropdown that sits next to the search bar. Lets the user pick
 * which installed region the map renders. When no region is ready (fresh
 * install or all regions in-progress) the trigger becomes a disabled
 * tooltip nudging the user toward the admin page.
 */

const ACTIVE_STATES: Region["state"][] = ["ready", "updating"];

function isActiveRegion(r: Region): boolean {
  return ACTIVE_STATES.includes(r.state);
}

function labelForRegion(r: Region): string {
  return r.displayName || r.name;
}

function keyForRegion(r: Region): string {
  return toCanonicalRegionKey(r.name);
}

export interface RegionSwitcherProps {
  /** Optional className forwarded to the trigger. */
  className?: string;
}

/** Google-Maps-style region chooser anchored to the top-left panel. */
export function RegionSwitcher({ className }: RegionSwitcherProps) {
  const installedRegions = useMapStore((s) => s.installedRegions);
  const activeRegion = useMapStore((s) => s.activeRegion);
  const setActiveRegion = useMapStore((s) => s.setActiveRegion);

  const ready = installedRegions.filter(isActiveRegion);
  const hasAny = ready.length > 0;

  const active = ready.find((r) => keyForRegion(r) === activeRegion) ?? null;
  const buttonLabel = !hasAny
    ? "No regions installed"
    : (active ? labelForRegion(active) : "All regions");

  const triggerClass = cn(
    "chrome-card pointer-events-auto inline-flex h-9 items-center gap-2 rounded-lg px-3 text-sm font-medium",
    "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
    "disabled:cursor-not-allowed disabled:opacity-60",
    className,
  );

  if (!hasAny) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            disabled
            aria-label="No regions installed"
            className={triggerClass}
          >
            <Globe className="h-4 w-4" aria-hidden="true" />
            <span>{buttonLabel}</span>
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom" align="start">
          Install via Admin → Regions
        </TooltipContent>
      </Tooltip>
    );
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          aria-label="Choose active region"
          className={triggerClass}
        >
          <Globe className="h-4 w-4" aria-hidden="true" />
          <span>{buttonLabel}</span>
          <ChevronDown className="h-4 w-4 opacity-70" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start" className="min-w-[14rem]">
        <DropdownMenuItem
          onSelect={() => setActiveRegion(null)}
          aria-label="Show all installed regions"
        >
          <span className="flex w-5 items-center justify-center">
            {activeRegion === null ? (
              <Check className="h-4 w-4" aria-hidden="true" />
            ) : null}
          </span>
          <span>All regions</span>
        </DropdownMenuItem>
        {ready.map((r) => {
          const k = keyForRegion(r);
          return (
            <DropdownMenuItem
              key={k}
              onSelect={() => setActiveRegion(k)}
              aria-label={`Switch to ${labelForRegion(r)}`}
            >
              <span className="flex w-5 items-center justify-center">
                {activeRegion === k ? (
                  <Check className="h-4 w-4" aria-hidden="true" />
                ) : null}
              </span>
              <span>{labelForRegion(r)}</span>
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
