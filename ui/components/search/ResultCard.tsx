"use client";

import { forwardRef } from "react";
import { cn } from "@/lib/utils";
import type { GeocodeResult } from "@/lib/api/schemas";
import { formatResult } from "@/lib/search/format-result";

export interface ResultCardProps {
  result: GeocodeResult;
  /** Reference point used to compute the distance badge. */
  origin?: { lat: number; lon: number } | null;
  /** Keyboard-driven highlight — drives the focus ring + aria-selected. */
  highlighted?: boolean;
  onSelect: (result: GeocodeResult) => void;
  /** Invoked when pointer enters the row; used to sync keyboard highlight. */
  onPointerOver?: (result: GeocodeResult) => void;
  /** The DOM id used by the combobox for `aria-activedescendant`. */
  id?: string;
}

/**
 * A single Google-Maps-style search result row.
 *
 * Layout: `[icon] [primary + secondary] [distance]`.
 * Roles:   the list wrapper is `role="listbox"`; each card is `role="option"`.
 *
 * Both Enter (keyboard via the SearchPanel handler) and click (pointer)
 * call `onSelect`. The row itself is a native `<button>` so screen
 * readers announce it as activatable without custom patterns.
 */
export const ResultCard = forwardRef<HTMLButtonElement, ResultCardProps>(
  function ResultCard(
    { result, origin, highlighted, onSelect, onPointerOver, id },
    ref,
  ) {
    const { icon: Icon, primary, secondary, distanceLabel } = formatResult(
      result,
      origin,
    );

    return (
      <button
        ref={ref}
        id={id}
        type="button"
        role="option"
        aria-selected={!!highlighted}
        data-highlighted={highlighted ? "true" : undefined}
        onClick={() => onSelect(result)}
        onPointerEnter={onPointerOver ? () => onPointerOver(result) : undefined}
        className={cn(
          "group flex w-full items-start gap-3 rounded-md px-3 py-2.5 text-left transition-colors",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          highlighted ? "bg-accent text-accent-foreground" : "hover:bg-muted",
        )}
      >
        <span
          className={cn(
            "mt-0.5 inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full",
            highlighted
              ? "bg-accent-foreground/10 text-accent-foreground"
              : "bg-muted text-muted-foreground",
          )}
          aria-hidden="true"
        >
          <Icon className="h-4 w-4" />
        </span>
        <span className="flex min-w-0 flex-1 flex-col">
          <span className="truncate text-sm font-medium text-foreground">
            {primary}
          </span>
          {secondary ? (
            <span className="truncate text-xs text-muted-foreground">
              {secondary}
            </span>
          ) : null}
        </span>
        {distanceLabel ? (
          <span
            className="mt-0.5 shrink-0 rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
            aria-label={`Distance: ${distanceLabel}`}
          >
            {distanceLabel}
          </span>
        ) : null}
      </button>
    );
  },
);
