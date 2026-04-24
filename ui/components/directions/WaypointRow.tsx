"use client";

import { useEffect, useRef, useState } from "react";
import {
  ArrowDown,
  ArrowUp,
  GripVertical,
  Loader2,
  X,
} from "lucide-react";
import type { GeocodeResult } from "@/lib/api/schemas";
import { useGeocodeAutocomplete } from "@/lib/api/hooks";
import { ResultCard } from "@/components/search/ResultCard";
import type { Waypoint } from "@/lib/state/directions";
import { cn } from "@/lib/utils";

/**
 * A single waypoint row: label, autocomplete search, remove, and
 * keyboard reorder alternatives to the visual drag handle. Uses
 * Agent J's `ResultCard` for the dropdown items so search styling
 * stays consistent across the app.
 */
export interface WaypointRowProps {
  waypoint: Waypoint;
  index: number;
  count: number;
  letter: string;
  canRemove: boolean;
  onChangeLabel: (label: string) => void;
  onSelect: (result: GeocodeResult) => void;
  onRemove: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onDragStart?: (index: number) => void;
  onDragOver?: (index: number) => void;
  onDrop?: () => void;
  focusLngLat?: { lat: number; lon: number };
}

export function WaypointRow({
  waypoint,
  index,
  count,
  letter,
  canRemove,
  onChangeLabel,
  onSelect,
  onRemove,
  onMoveUp,
  onMoveDown,
  onDragStart,
  onDragOver,
  onDrop,
  focusLngLat,
}: WaypointRowProps) {
  const [open, setOpen] = useState(false);
  const rowRef = useRef<HTMLLIElement | null>(null);

  const ac = useGeocodeAutocomplete({
    q: waypoint.label,
    focus: focusLngLat,
    enabled: open && !waypoint.lngLat,
  });
  const results = ac.data?.results ?? [];

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (rowRef.current && !rowRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open]);

  return (
    <li
      ref={rowRef}
      role="listitem"
      draggable
      onDragStart={(e) => {
        e.dataTransfer.effectAllowed = "move";
        onDragStart?.(index);
      }}
      onDragOver={(e) => {
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        onDragOver?.(index);
      }}
      onDrop={(e) => {
        e.preventDefault();
        onDrop?.();
      }}
      className="group relative flex items-start gap-2 rounded-md px-1 py-1 hover:bg-muted/50"
    >
      <span
        aria-hidden={true}
        className="mt-2 inline-flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full bg-primary/10 text-xs font-semibold text-primary"
      >
        {letter}
      </span>

      <div className="flex min-w-0 flex-1 flex-col">
        <div className="flex items-center gap-1">
          <input
            type="text"
            aria-label={`Waypoint ${letter}`}
            placeholder={waypoint.placeholder ?? "Search place"}
            value={waypoint.label}
            onChange={(e) => {
              onChangeLabel(e.target.value);
              setOpen(true);
            }}
            onFocus={() => setOpen(true)}
            className={cn(
              "flex-1 rounded-md border border-transparent bg-background px-2 py-1.5 text-sm focus:border-ring focus:outline-none",
              !waypoint.lngLat && "text-foreground",
            )}
          />
          {ac.isFetching && (
            <Loader2
              className="h-4 w-4 animate-spin text-muted-foreground"
              aria-label="Searching"
            />
          )}
          {canRemove && (
            <button
              type="button"
              aria-label="Remove waypoint"
              onClick={onRemove}
              className="rounded p-1 text-muted-foreground hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <X className="h-4 w-4" aria-hidden={true} />
            </button>
          )}
        </div>

        {open && results.length > 0 && (
          <div
            role="listbox"
            aria-label="Search results"
            className="mt-1 max-h-60 overflow-auto rounded-md border border-border bg-popover text-sm shadow-sm"
          >
            {results.map((r) => (
              <ResultCard
                key={r.id}
                result={r}
                origin={focusLngLat ?? undefined}
                onSelect={(picked) => {
                  onSelect(picked);
                  setOpen(false);
                }}
              />
            ))}
          </div>
        )}
      </div>

      <div
        className="ml-1 mt-1 flex flex-shrink-0 flex-col items-center gap-0.5"
        aria-label={`Reorder waypoint ${letter}`}
      >
        <button
          type="button"
          aria-label="Move waypoint up"
          onClick={onMoveUp}
          disabled={index === 0}
          className="rounded p-0.5 text-muted-foreground hover:text-foreground disabled:opacity-30 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ArrowUp className="h-3 w-3" aria-hidden={true} />
        </button>
        <span
          aria-hidden={true}
          className="cursor-grab text-muted-foreground"
          title="Drag to reorder"
        >
          <GripVertical className="h-4 w-4" />
        </span>
        <button
          type="button"
          aria-label="Move waypoint down"
          onClick={onMoveDown}
          disabled={index === count - 1}
          className="rounded p-0.5 text-muted-foreground hover:text-foreground disabled:opacity-30 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <ArrowDown className="h-3 w-3" aria-hidden={true} />
        </button>
      </div>
    </li>
  );
}
