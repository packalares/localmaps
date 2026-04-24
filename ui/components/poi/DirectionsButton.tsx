"use client";

import { Navigation } from "lucide-react";
import type { Poi } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";

export interface DirectionsButtonProps {
  poi: Poi;
  /**
   * Called when the user asks for directions to this POI. Primary merge
   * wires this into the directions slice (sets the last waypoint to
   * `{lngLat, label}` and switches the left-rail tab to Directions).
   *
   * TODO: switch to K's directions slice dispatch once it exists.
   */
  onDirections?: (poi: Poi) => void;
  className?: string;
}

/**
 * Prominent "Directions" button modelled on Google Maps' green pill.
 * Kept as its own file so the dispatch target can be swapped out
 * without touching ActionRow when K's slice lands.
 */
export function DirectionsButton({
  poi,
  onDirections,
  className,
}: DirectionsButtonProps) {
  return (
    <button
      type="button"
      onClick={() => onDirections?.(poi)}
      aria-label={`Directions to ${poi.label || "this place"}`}
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full bg-primary px-4 py-1.5 text-sm font-medium text-primary-foreground shadow-chrome transition-colors hover:bg-primary/90 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
        className,
      )}
    >
      <Navigation className="h-4 w-4" aria-hidden="true" />
      <span>Directions</span>
    </button>
  );
}
