"use client";

import { X } from "lucide-react";
import type { Poi } from "@/lib/api/schemas";
import { iconForPoi, primaryText, secondaryText } from "@/lib/poi/format-poi";
import { cn } from "@/lib/utils";

export interface PoiHeaderProps {
  poi: Pick<Poi, "label" | "category" | "tags">;
  onClose?: () => void;
  className?: string;
}

/**
 * Top of the POI pane: large title, a subtitle line with the place's
 * type icon + text, and a close button. Uses an h2 heading so it slots
 * under the app-level h1.
 */
export function PoiHeader({ poi, onClose, className }: PoiHeaderProps) {
  const Icon = iconForPoi({ category: poi.category, tags: poi.tags });
  const title = primaryText({ label: poi.label, category: poi.category });
  const subtitle = secondaryText({ category: poi.category, tags: poi.tags });

  return (
    <header
      className={cn(
        "flex items-start justify-between gap-3 border-b border-border px-4 pb-3 pt-4",
        className,
      )}
    >
      <div className="min-w-0 flex-1">
        <h2 className="truncate text-xl font-semibold text-foreground">
          {title}
        </h2>
        {subtitle && (
          <p className="mt-1 flex items-center gap-1.5 text-sm text-muted-foreground">
            <Icon className="h-4 w-4 shrink-0" aria-hidden="true" />
            <span className="truncate">{subtitle}</span>
          </p>
        )}
      </div>

      {onClose && (
        <button
          type="button"
          onClick={onClose}
          aria-label="Close place details"
          className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-muted-foreground hover:bg-muted hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </button>
      )}
    </header>
  );
}
