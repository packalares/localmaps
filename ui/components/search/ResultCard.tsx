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
          // Layout: [icon] [primary + secondary] [distance]. Single line
          // 56-64px tall via py-3.
          "group flex w-full items-center gap-3 rounded-md px-3 py-3 text-left transition-colors",
          "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          // Subtle hover/highlight only — no big filled blue card.
          // Highlighted state uses a faint primary tint + an inset ring
          // so keyboard focus is still visible without flooding the row.
          highlighted
            ? "bg-primary/5 ring-1 ring-inset ring-primary/30"
            : "hover:bg-muted",
        )}
      >
        <span
          className={cn(
            "inline-flex h-5 w-5 shrink-0 items-center justify-center text-muted-foreground",
          )}
          aria-hidden="true"
        >
          <Icon className="h-5 w-5" />
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
        {/* Distance label intentionally hidden — it noise-trips on every
            map pan/zoom and the user explicitly asked to drop it. */}
      </button>
    );
  },
);
