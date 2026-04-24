"use client";

import { useMemo } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import {
  formatMeasureArea,
  formatMeasureDistance,
  polygonAreaMetres,
  polylineDistanceMetres,
} from "@/lib/tools/geometry";
import { useMeasureStore } from "@/lib/tools/measure-state";

export interface MeasureOverlayProps {
  /** Units honour the user's setting; default metric matches the charter. */
  units?: "metric" | "imperial";
}

/**
 * Floating summary chip for the live measure tool. Renders only when the
 * measure tool is the active tool. Announces distance/area changes to an
 * aria-live region for assistive tech.
 */
export function MeasureOverlay({ units = "metric" }: MeasureOverlayProps) {
  const active = useActiveToolStore((s) => s.active);
  const closeAll = useActiveToolStore((s) => s.closeAll);
  const setMode = useMeasureStore((s) => s.setMode);
  const mode = useMeasureStore((s) => s.mode);
  const points = useMeasureStore((s) => s.points);
  const finalised = useMeasureStore((s) => s.isFinalised);

  const distance = useMemo(() => polylineDistanceMetres(points), [points]);
  const area = useMemo(() => polygonAreaMetres(points), [points]);

  if (active !== "measure") return null;

  const primary =
    mode === "area"
      ? points.length >= 3
        ? formatMeasureArea(area, units)
        : "Tap 3+ points to outline an area"
      : points.length >= 2
        ? formatMeasureDistance(distance, units)
        : "Tap to add a point";
  const hint = finalised
    ? "Finalised. Press Esc to close."
    : "Double-click or Enter to finish · Backspace to undo";

  return (
    <div
      className="pointer-events-auto chrome-card absolute bottom-20 left-1/2 z-20 -translate-x-1/2 px-4 py-3 text-sm shadow-lg"
      role="dialog"
      aria-label="Measure tool"
    >
      <div className="flex items-center gap-3">
        <div
          className="inline-flex rounded-md border border-border text-xs"
          role="tablist"
          aria-label="Measure mode"
        >
          <button
            type="button"
            role="tab"
            aria-selected={mode === "distance"}
            onClick={() => setMode("distance")}
            className={
              mode === "distance"
                ? "bg-primary px-3 py-1 text-primary-foreground rounded-l-md"
                : "px-3 py-1 hover:bg-muted rounded-l-md"
            }
          >
            Distance
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={mode === "area"}
            onClick={() => setMode("area")}
            className={
              mode === "area"
                ? "bg-primary px-3 py-1 text-primary-foreground rounded-r-md"
                : "px-3 py-1 hover:bg-muted rounded-r-md"
            }
          >
            Area
          </button>
        </div>
        <div className="min-w-[180px] tabular-nums">
          <div
            className="font-semibold"
            role="status"
            aria-live="polite"
            aria-atomic="true"
          >
            {primary}
          </div>
          <div className="text-xs text-muted-foreground">{hint}</div>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={closeAll}
          aria-label="Close measure tool"
          title="Close"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>
    </div>
  );
}
