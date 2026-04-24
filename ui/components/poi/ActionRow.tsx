"use client";

import { Globe, Phone, Share2, Bookmark } from "lucide-react";
import type { Poi } from "@/lib/api/schemas";
import { phoneOf, websiteOf } from "@/lib/poi/format-poi";
import { cn } from "@/lib/utils";
import { DirectionsButton } from "./DirectionsButton";
import { useMessages } from "@/lib/i18n/provider";

export interface ActionRowProps {
  poi: Poi;
  onShare?: (poi: Poi) => void;
  onSave?: (poi: Poi) => void;
  onDirections?: (poi: Poi) => void;
  className?: string;
}

/**
 * Google Maps-style action row under the POI header: Directions is the
 * visually-prominent primary action; phone / website / share / save
 * are secondary chip buttons.
 */
export function ActionRow({
  poi,
  onShare,
  onSave,
  onDirections,
  className,
}: ActionRowProps) {
  const phone = phoneOf(poi.tags);
  const website = websiteOf(poi.tags);
  const { t } = useMessages();

  return (
    <div
      className={cn(
        "flex flex-wrap items-center gap-2 border-b border-border px-4 py-3",
        className,
      )}
      role="group"
      aria-label="Place actions"
    >
      <DirectionsButton poi={poi} onDirections={onDirections} />

      {phone && (
        <a
          href={`tel:${phone.replace(/[^+\d]/g, "")}`}
          className="inline-flex items-center gap-1.5 rounded-full border border-border bg-background px-3 py-1.5 text-sm hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label={`Call ${phone}`}
        >
          <Phone className="h-4 w-4" aria-hidden="true" />
          <span>{t("poi.action.call")}</span>
        </a>
      )}

      {website && (
        <a
          href={website}
          target="_blank"
          rel="noreferrer noopener"
          className="inline-flex items-center gap-1.5 rounded-full border border-border bg-background px-3 py-1.5 text-sm hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          aria-label="Visit website"
        >
          <Globe className="h-4 w-4" aria-hidden="true" />
          <span>{t("poi.action.website")}</span>
        </a>
      )}

      <button
        type="button"
        onClick={() => onShare?.(poi)}
        className="inline-flex items-center gap-1.5 rounded-full border border-border bg-background px-3 py-1.5 text-sm hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Share2 className="h-4 w-4" aria-hidden="true" />
        <span>{t("poi.action.share")}</span>
      </button>

      <button
        type="button"
        onClick={() => onSave?.(poi)}
        className="inline-flex items-center gap-1.5 rounded-full border border-border bg-background px-3 py-1.5 text-sm hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      >
        <Bookmark className="h-4 w-4" aria-hidden="true" />
        <span>{t("poi.action.save")}</span>
      </button>
    </div>
  );
}
