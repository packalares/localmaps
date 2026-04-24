"use client";

import {
  ArrowDown,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  CornerDownLeft,
  CornerDownRight,
  CornerUpLeft,
  CornerUpRight,
  Flag,
  MapPin,
  Merge,
  Undo2,
} from "lucide-react";
import type { RouteLeg } from "@/lib/api/schemas";
import { formatDistance } from "@/lib/directions/format-distance";
import { cn } from "@/lib/utils";

export type Maneuver = RouteLeg["maneuvers"][number];

export interface ManeuverRowProps {
  maneuver: Maneuver;
  index: number;
  active?: boolean;
  onActivate?: (index: number) => void;
  units?: "metric" | "imperial";
}

/**
 * Maps Valhalla-ish type codes to a lucide icon. Unknown types fall
 * back to a straight arrow. Valhalla `type` is a numeric code in the
 * wire format; callers can normalise upstream or just pass strings
 * — we accept both by coercion.
 */
function iconFor(type: string | undefined) {
  const t = (type ?? "").toLowerCase();
  if (t.includes("u-turn") || t.includes("uturn")) return Undo2;
  if (t.includes("merge")) return Merge;
  if (t.includes("sharp_left") || t.includes("sharp-left"))
    return CornerUpLeft;
  if (t.includes("sharp_right") || t.includes("sharp-right"))
    return CornerUpRight;
  if (t.includes("slight_left") || t.includes("slight-left"))
    return CornerDownLeft;
  if (t.includes("slight_right") || t.includes("slight-right"))
    return CornerDownRight;
  if (t === "left" || t.includes("left")) return ArrowLeft;
  if (t === "right" || t.includes("right")) return ArrowRight;
  if (t.includes("arrive") || t.includes("destination")) return Flag;
  if (t.includes("depart") || t.includes("start")) return MapPin;
  if (t.includes("down")) return ArrowDown;
  return ArrowUp;
}

export function ManeuverRow({
  maneuver,
  index,
  active,
  onActivate,
  units,
}: ManeuverRowProps) {
  const Icon = iconFor(maneuver.type);
  const distance = maneuver.distanceMeters;
  const street = maneuver.streetName ?? "";

  return (
    <li
      role="listitem"
      aria-current={active ? "step" : undefined}
      className={cn(
        "flex items-start gap-3 rounded-md px-2 py-2 text-sm",
        active ? "bg-primary/10" : "hover:bg-muted",
      )}
    >
      <button
        type="button"
        onClick={() => onActivate?.(index)}
        aria-label={`Maneuver ${index + 1}: ${maneuver.instruction}`}
        className="flex w-full items-start gap-3 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span
          aria-hidden={true}
          className="mt-0.5 inline-flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full bg-primary/10 text-primary"
        >
          <Icon className="h-4 w-4" aria-hidden={true} />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate font-medium text-foreground">
            {maneuver.instruction}
          </span>
          {street && (
            <span className="truncate text-xs text-muted-foreground">
              {street}
            </span>
          )}
        </span>
        {typeof distance === "number" && distance > 0 && (
          <span className="ml-2 flex-shrink-0 text-xs text-muted-foreground">
            {formatDistance(distance, { units })}
          </span>
        )}
      </button>
    </li>
  );
}
