"use client";

import { Bike, Car, Footprints, Truck, X, type LucideIcon } from "lucide-react";
import type { Route, RouteMode } from "@/lib/api/schemas";
import { formatDistance } from "@/lib/directions/format-distance";
import { formatDuration } from "@/lib/directions/format-duration";
import { cn } from "@/lib/utils";

export interface RouteSummaryProps {
  route: Route;
  alternatives: Route[];
  onSelectAlternative?: (routeId: string) => void;
  onClear?: () => void;
  units?: "metric" | "imperial";
}

const MODE_ICON: Record<RouteMode, LucideIcon> = {
  auto: Car,
  bicycle: Bike,
  pedestrian: Footprints,
  truck: Truck,
};

function deriveMainRoad(route: Route): string | null {
  for (const leg of route.legs) {
    for (const m of leg.maneuvers) {
      if (m.streetName && m.streetName.trim().length > 0) return m.streetName;
    }
  }
  return null;
}

export function RouteSummary({
  route,
  alternatives,
  onSelectAlternative,
  onClear,
  units,
}: RouteSummaryProps) {
  const time = route.summary?.timeSeconds ?? 0;
  const dist = route.summary?.distanceMeters ?? 0;
  const mode = route.mode ?? "auto";
  const Icon = MODE_ICON[mode];
  const via = deriveMainRoad(route);

  return (
    <div className="rounded-md border border-border bg-background px-3 py-3">
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0 flex-1">
          <p className="text-2xl font-semibold leading-tight text-foreground">
            {formatDuration(time)}
          </p>
          <p className="text-sm text-muted-foreground">
            {formatDistance(dist, { units })}
            {via ? ` · Via ${via}` : ""}
          </p>
          <p className="mt-1 flex items-center gap-1 text-xs text-muted-foreground">
            <Icon className="h-3.5 w-3.5" aria-hidden={true} />
            <span className="capitalize">{mode === "auto" ? "driving" : mode}</span>
          </p>
        </div>
        {onClear && (
          <button
            type="button"
            aria-label="Clear route"
            onClick={onClear}
            className="rounded-md p-1 text-muted-foreground hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <X className="h-4 w-4" aria-hidden={true} />
          </button>
        )}
      </div>

      {alternatives.length > 1 && (
        <div
          role="group"
          aria-label="Alternative routes"
          className="mt-3 flex flex-wrap gap-2"
        >
          {alternatives.map((alt, i) => {
            const active = alt.id === route.id;
            return (
              <button
                key={alt.id}
                type="button"
                aria-pressed={active}
                onClick={() => onSelectAlternative?.(alt.id)}
                className={cn(
                  "rounded-full border px-3 py-1 text-xs transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  active
                    ? "border-primary bg-primary/10 text-primary"
                    : "border-border text-muted-foreground hover:bg-muted",
                )}
              >
                {formatDuration(alt.summary?.timeSeconds ?? 0)} · alt {i + 1}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
