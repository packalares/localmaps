"use client";

import { useEffect, useRef } from "react";
import type { Poi } from "@/lib/api/schemas";
import { addressLines } from "@/lib/poi/format-poi";
import { cn } from "@/lib/utils";
import { ActionRow } from "./ActionRow";
import { HoursAccordion } from "./HoursAccordion";
import { PoiHeader } from "./PoiHeader";
import { TagTable } from "./TagTable";
import { useMessages } from "@/lib/i18n/provider";

export type PoiPaneStatus = "idle" | "loading" | "error";

export interface PoiPaneProps {
  /** The resolved POI; when null + status=loading, a skeleton renders. */
  poi: Poi | null;
  status?: PoiPaneStatus;
  /** Called when the user dismisses the pane (X button or Escape). */
  onClose?: () => void;
  onDirections?: (poi: Poi) => void;
  onShare?: (poi: Poi) => void;
  onSave?: (poi: Poi) => void;
  /** Testing seam for "now" used by the hours accordion. */
  now?: Date;
  className?: string;
}

/**
 * Left-rail POI details pane. Mirrors Google Maps' place card:
 *
 *   ┌──────────────────────────────┐
 *   │  Title                    ✕  │
 *   │  icon  Type                  │
 *   ├──────────────────────────────┤
 *   │  [Directions] [Call] [Web]…  │
 *   ├──────────────────────────────┤
 *   │  Address                     │
 *   ├──────────────────────────────┤
 *   │  🕑 Open now · closes 22:00 ⌄│
 *   ├──────────────────────────────┤
 *   │  Tags (collapsed)            │
 *   └──────────────────────────────┘
 *
 * Escape key dismisses the pane when it has focus; the caller is
 * responsible for rendering it inside the left-rail tab area.
 */
export function PoiPane({
  poi,
  status = "idle",
  onClose,
  onDirections,
  onShare,
  onSave,
  now,
  className,
}: PoiPaneProps) {
  const paneRef = useRef<HTMLDivElement | null>(null);
  const { t } = useMessages();

  // Focus the pane when a POI loads so screen readers land in the card
  // and Escape starts working immediately.
  const poiId = poi?.id ?? null;
  useEffect(() => {
    if (poiId && paneRef.current) {
      paneRef.current.focus();
    }
  }, [poiId]);

  // Escape closes the pane when the user's focus is inside it.
  useEffect(() => {
    if (!poi && status !== "error") return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      const el = paneRef.current;
      if (el && (el === document.activeElement || el.contains(document.activeElement))) {
        e.preventDefault();
        onClose?.();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [poi, status, onClose]);

  if (status === "loading" && !poi) {
    return (
      <div
        ref={paneRef}
        tabIndex={-1}
        role="status"
        aria-live="polite"
        aria-label="Loading place details"
        className={cn("flex flex-col gap-3 p-4", className)}
      >
        <div className="h-5 w-3/4 animate-pulse rounded bg-muted" />
        <div className="h-4 w-1/2 animate-pulse rounded bg-muted" />
        <div className="mt-2 h-8 w-40 animate-pulse rounded-full bg-muted" />
        <div className="mt-3 space-y-2">
          <div className="h-3 w-full animate-pulse rounded bg-muted" />
          <div className="h-3 w-5/6 animate-pulse rounded bg-muted" />
        </div>
      </div>
    );
  }

  if (status === "error" && !poi) {
    return (
      <div
        ref={paneRef}
        tabIndex={-1}
        role="alert"
        className={cn("flex flex-col gap-2 p-4 text-sm", className)}
      >
        <p className="font-medium">{t("poi.error.title")}</p>
        <p className="text-muted-foreground">{t("poi.error.body")}</p>
        {onClose && (
          <button
            type="button"
            onClick={onClose}
            className="mt-2 self-start rounded border border-border px-3 py-1 text-sm hover:bg-muted"
          >
            {t("common.close")}
          </button>
        )}
      </div>
    );
  }

  if (!poi) return null;

  const tags = poi.tags ?? {};
  const lines = addressLines(tags);

  return (
    <div
      ref={paneRef}
      tabIndex={-1}
      role="region"
      aria-label={`Details for ${poi.label || "place"}`}
      className={cn(
        "flex h-full flex-col overflow-y-auto focus:outline-none",
        className,
      )}
    >
      <PoiHeader poi={poi} onClose={onClose} />
      <ActionRow
        poi={poi}
        onDirections={onDirections}
        onShare={onShare}
        onSave={onSave}
      />

      {lines.length > 0 && (
        <section
          aria-label="Address"
          className="border-b border-border px-4 py-3 text-sm"
        >
          {lines.map((line) => (
            <div key={line} className="leading-snug text-foreground">
              {line}
            </div>
          ))}
        </section>
      )}

      <HoursAccordion raw={tags["opening_hours"]} now={now} />

      <TagTable tags={tags} />
    </div>
  );
}
