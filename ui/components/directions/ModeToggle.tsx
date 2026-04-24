"use client";

import { Bike, Car, Footprints, Truck, type LucideIcon } from "lucide-react";
import type { RouteMode } from "@/lib/api/schemas";
import { cn } from "@/lib/utils";

export interface ModeToggleProps {
  value: RouteMode;
  onChange: (value: RouteMode) => void;
  /** Modes to render; defaults to all four. Primary passes this from settings. */
  modes?: RouteMode[];
}

interface ModeConfig {
  mode: RouteMode;
  label: string;
  icon: LucideIcon;
}

const CONFIGS: ModeConfig[] = [
  { mode: "auto", label: "Driving", icon: Car },
  { mode: "bicycle", label: "Cycling", icon: Bike },
  { mode: "pedestrian", label: "Walking", icon: Footprints },
  { mode: "truck", label: "Truck", icon: Truck },
];

export function ModeToggle({ value, onChange, modes }: ModeToggleProps) {
  const enabled = modes ?? ["auto", "bicycle", "pedestrian", "truck"];
  const visible = CONFIGS.filter((c) => enabled.includes(c.mode));

  return (
    <div
      role="tablist"
      aria-label="Travel mode"
      className="flex items-center gap-1 rounded-full bg-muted p-1"
    >
      {visible.map(({ mode, label, icon: Icon }) => {
        const active = mode === value;
        return (
          <button
            key={mode}
            type="button"
            role="tab"
            aria-selected={active}
            aria-label={label}
            title={label}
            onClick={() => onChange(mode)}
            className={cn(
              "inline-flex h-8 items-center gap-1.5 rounded-full px-3 text-xs font-medium transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
              active
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground",
            )}
          >
            <Icon className="h-4 w-4" aria-hidden={true} />
            <span>{label}</span>
          </button>
        );
      })}
    </div>
  );
}
