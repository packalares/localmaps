"use client";

import { Ruler, Timer, X as XIcon } from "lucide-react";
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover";
import { cn } from "@/lib/utils";
import { useActiveToolStore } from "@/lib/tools/active-tool";

/**
 * Floating action button that opens a Radix popover with the Phase-7
 * tool suite. Renders the individual tool drivers (`MeasureTool`,
 * `IsochroneTool`) and their UI companions (`MeasureOverlay`,
 * `IsochronePanel`) as siblings, not children, so they stay mounted
 * while the popover closes — the popover itself is only the
 * chooser-affordance.
 *
 * Active-state indicator: when a tool is running, the FAB renders a
 * small badge and the trigger's aria-label includes the tool name for
 * screen readers.
 */
export function ToolsFab() {
  const active = useActiveToolStore((s) => s.active);
  const setActive = useActiveToolStore((s) => s.setActive);
  const closeAll = useActiveToolStore((s) => s.closeAll);

  const label =
    active === "measure"
      ? "Tools (Measure active)"
      : active === "isochrone"
        ? "Tools (Isochrone active)"
        : "Tools";

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button
          type="button"
          aria-label={label}
          title={label}
          className={cn(
            "relative inline-flex h-8 w-8 items-center justify-center rounded-lg bg-white text-foreground",
            "shadow-sm ring-1 ring-black/10 hover:bg-neutral-100",
            "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
            "dark:bg-neutral-900 dark:ring-white/10 dark:hover:bg-neutral-800",
            active && "ring-2 ring-primary",
          )}
        >
          <Ruler className="h-4 w-4" aria-hidden="true" />
          {active && (
            <span
              aria-hidden="true"
              className="absolute -right-1 -top-1 h-2 w-2 rounded-full bg-primary"
            />
          )}
        </button>
      </PopoverTrigger>
      <PopoverContent side="left" align="start" className="w-64 p-2">
        <div className="mb-1 px-2 text-xs font-medium text-muted-foreground">
          Tools
        </div>
        <button
          type="button"
          onClick={() => setActive("measure")}
          aria-pressed={active === "measure"}
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
        >
          <Ruler className="h-4 w-4" aria-hidden="true" />
          <span className="flex-1">
            <span className="block font-medium">Measure</span>
            <span className="block text-xs text-muted-foreground">
              Distance or area
            </span>
          </span>
        </button>
        <button
          type="button"
          onClick={() => setActive("isochrone")}
          aria-pressed={active === "isochrone"}
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted"
        >
          <Timer className="h-4 w-4" aria-hidden="true" />
          <span className="flex-1">
            <span className="block font-medium">Travel time</span>
            <span className="block text-xs text-muted-foreground">
              10 / 15 / 30-minute isochrones
            </span>
          </span>
        </button>
        <div className="my-1 border-t border-border" />
        <button
          type="button"
          onClick={closeAll}
          className="flex w-full items-center gap-2 rounded-md px-3 py-2 text-left text-sm hover:bg-muted disabled:opacity-50"
          disabled={!active}
        >
          <XIcon className="h-4 w-4" aria-hidden="true" />
          Close all
        </button>
      </PopoverContent>
    </Popover>
  );
}
