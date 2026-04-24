"use client";

import type { Route } from "@/lib/api/schemas";
import { ManeuverRow } from "./ManeuverRow";

export interface TurnListProps {
  route: Route;
  /** Index of the maneuver currently being centred on the map, if any. */
  activeIndex?: number;
  /** Called with the maneuver index when the user clicks a row. */
  onSelect?: (index: number) => void;
  units?: "metric" | "imperial";
}

/**
 * Renders every maneuver in the route's legs as a single flat ordered
 * list. The `beginShapeIndex` of each maneuver references the decoded
 * polyline — callers use this to pan the map to the corresponding
 * point.
 */
export function TurnList({
  route,
  activeIndex,
  onSelect,
  units,
}: TurnListProps) {
  const flat = route.legs.flatMap((leg) => leg.maneuvers);

  if (flat.length === 0) {
    return (
      <p className="px-3 py-4 text-sm text-muted-foreground">
        No turn-by-turn instructions were returned for this route.
      </p>
    );
  }

  return (
    <ol role="list" aria-label="Turn-by-turn instructions" className="px-1 py-1">
      {flat.map((m, i) => (
        <ManeuverRow
          key={`${i}-${m.beginShapeIndex}`}
          maneuver={m}
          index={i}
          active={activeIndex === i}
          onActivate={onSelect}
          units={units}
        />
      ))}
    </ol>
  );
}
