"use client";

import { useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";
import type { RouteOptions as RouteOptionsValue } from "@/lib/state/directions";
import { cn } from "@/lib/utils";

export interface RouteOptionsProps {
  value: RouteOptionsValue;
  onChange: (next: Partial<RouteOptionsValue>) => void;
}

/**
 * Expander with the three avoid toggles (highways / tolls / ferries).
 * Collapsed by default to keep the panel compact, matching Google Maps.
 */
export function RouteOptions({ value, onChange }: RouteOptionsProps) {
  const [open, setOpen] = useState(false);

  return (
    <div className="rounded-md border border-border bg-background/60">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center justify-between px-3 py-2 text-xs font-medium text-muted-foreground hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <span>Route options</span>
        {open ? (
          <ChevronUp className="h-4 w-4" aria-hidden={true} />
        ) : (
          <ChevronDown className="h-4 w-4" aria-hidden={true} />
        )}
      </button>
      <div
        className={cn(
          "grid gap-2 overflow-hidden px-3 transition-all",
          open ? "max-h-40 py-2" : "max-h-0",
        )}
      >
        <Toggle
          id="avoid-highways"
          label="Avoid highways"
          checked={value.avoidHighways}
          onChange={(v) => onChange({ avoidHighways: v })}
        />
        <Toggle
          id="avoid-tolls"
          label="Avoid tolls"
          checked={value.avoidTolls}
          onChange={(v) => onChange({ avoidTolls: v })}
        />
        <Toggle
          id="avoid-ferries"
          label="Avoid ferries"
          checked={value.avoidFerries}
          onChange={(v) => onChange({ avoidFerries: v })}
        />
      </div>
    </div>
  );
}

function Toggle({
  id,
  label,
  checked,
  onChange,
}: {
  id: string;
  label: string;
  checked: boolean;
  onChange: (next: boolean) => void;
}) {
  return (
    <label htmlFor={id} className="flex items-center gap-2 text-sm">
      <input
        id={id}
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="h-4 w-4 accent-primary"
      />
      <span>{label}</span>
    </label>
  );
}
