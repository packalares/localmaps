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
 * Tiny dropdown that sits next to the search bar.
 *
 * As of the multi-region tile rollout the map renders ALL installed
 * regions at once via the bbox-based tile-router — there's no longer a
 * concept of "which region is visible". What this dropdown still
 * meaningfully controls:
 *
 *   * The viewport auto-fits to the chosen region's bbox on first
 *     selection (zoom-to-Romania, zoom-to-Greece, …).
 *   * Routing and search APIs still operate against ONE region at a
 *     time — selecting from this dropdown also picks which region
 *     answers "directions from X to Y" and "search Athens" queries.
 *
 * The label "Home region" makes both purposes obvious; the dropdown
 * tooltip spells out the trade-off so a user who toggles regions
 * understands why a search starts returning different results.
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
  // Two-line trigger: a small caption "Home" + the active region name
  // makes it clear this is the home / routing region. When no region
  // is selected, fall back to "All regions" because routing endpoints
  // will return a "pick a region" hint and the tile layer renders
  // everything anyway.
  const buttonLabel = !hasAny
    ? "No regions installed"
    : (active ? labelForRegion(active) : "Pick a home region");

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
      <Tooltip>
        <TooltipTrigger asChild>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              aria-label="Pick the home region used for routing and search"
              className={triggerClass}
            >
              <Globe className="h-4 w-4" aria-hidden="true" />
              <span>{buttonLabel}</span>
              <ChevronDown
                className="h-4 w-4 opacity-70"
                aria-hidden="true"
              />
            </button>
          </DropdownMenuTrigger>
        </TooltipTrigger>
        <TooltipContent side="bottom" align="start" className="max-w-xs">
          The map already shows every installed region. This picks the
          region used for routing and search — the gateway answers
          "directions" and "search address" queries against ONE region
          at a time.
        </TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="start" className="min-w-[14rem]">
        <DropdownMenuItem
          onSelect={() => setActiveRegion(null)}
          aria-label="Clear home region selection"
        >
          <span className="flex w-5 items-center justify-center">
            {activeRegion === null ? (
              <Check className="h-4 w-4" aria-hidden="true" />
            ) : null}
          </span>
          <span>No home region</span>
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
