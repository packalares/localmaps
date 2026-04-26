"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import maplibregl from "maplibre-gl";
import {
  Clock,
  Globe,
  Navigation2,
  Phone,
  Share2,
  X,
} from "lucide-react";
import { ShareDialog } from "@/components/share/ShareDialog";
import type { SelectedFeature } from "@/lib/state/place";
import { usePlaceStore } from "@/lib/state/place";
import { useMapStore } from "@/lib/state/map";
import { useDirectionsStore } from "@/lib/state/directions";
import { useReverseGeocode, usePoi } from "@/lib/api/hooks";
import { apiUrl } from "@/lib/env";
import { cn } from "@/lib/utils";
import { makeDroppedPinElement } from "@/components/chrome/marker-elements";

/**
 * Bottom-center info card shown when the user clicks the map:
 *
 *   ┌──────────────────────────────────────────────────────────┐
 *   │ [thumb]  Primary line (address or POI name)              │
 *   │  80×80   Subtitle (city · country OR category)           │
 *   │          44.47992, 26.16213       [↗][⇅][×]              │
 *   └──────────────────────────────────────────────────────────┘
 *
 * POI kind expands the subtitle with opening hours / phone / website
 * when they are available on `tags`; a plain point kind falls back to
 * the reverse-geocode label for the subtitle.
 *
 * The component is position:fixed at the bottom-center of the
 * viewport; the wrapping container in `app/page.tsx` gives it the
 * pointer-events-auto layer and the top/bottom safe-area margins.
 */
export interface PointInfoCardProps {
  /** The feature that was clicked. When null the card hides itself. */
  feature: SelectedFeature | null;
  /** Invoked when the user hits the X or presses Escape. */
  onClose: () => void;
  /**
   * Invoked when the user hits the blue "Directions" CTA.
   * Default behaviour sets the directions-store destination and flips
   * the left rail to the directions tab.
   */
  onDirections?: (feature: SelectedFeature) => void;
  /** Invoked when the user hits the share button. */
  onShare?: (feature: SelectedFeature) => void;
  className?: string;
}

export function PointInfoCard({
  feature,
  onClose,
  onDirections,
  onShare,
  className,
}: PointInfoCardProps) {
  // Share opens the rich 3-tab dialog (Link / QR / Embed) used by the
  // top-bar share affordance. Before opening, replace the URL with a
  // deep-link to the selected feature so the dialog (which reads
  // window.location) shares the clicked point, not just the viewport.
  const [shareOpen, setShareOpen] = useState(false);
  const handleShare = useCallback(
    (f: SelectedFeature) => {
      if (onShare) {
        onShare(f);
        return;
      }
      if (typeof window !== "undefined") {
        const url = buildShareLink(window.location.host, f);
        const path = url.replace(/^https?:\/\/[^/]+/, "");
        try {
          window.history.replaceState(null, "", path);
        } catch {
          /* iframe/sandbox — fall through to dialog with current URL */
        }
      }
      setShareOpen(true);
    },
    [onShare],
  );

  // Reverse geocode runs only for plain points; POIs bring their own
  // address through `usePoi` below.
  const reverse = useReverseGeocode({
    lngLat: feature && feature.kind === "point"
      ? { lng: feature.lon, lat: feature.lat }
      : null,
    enabled: !!feature && feature.kind === "point" && !feature.address,
  });

  // POI fetch: enabled only when we have a POI id. The endpoint 501s
  // in unit tests, which is fine — `feature.name` carries the click-
  // time fallback so the header still renders.
  const poiQuery = usePoi(
    feature?.kind === "poi" ? feature.id ?? null : null,
    { enabled: !!feature && feature.kind === "poi" && !!feature.id },
  );

  const primary = useMemo(() => {
    if (!feature) return "";
    if (feature.name) return feature.name;
    if (feature.kind === "poi") {
      return poiQuery.data?.label ?? "Unnamed place";
    }
    return reverse.data?.result.label ?? feature.address ?? "Unnamed place";
  }, [feature, poiQuery.data, reverse.data]);

  const secondary = useMemo(() => {
    if (!feature) return "";
    if (feature.kind === "poi") {
      const poi = poiQuery.data;
      const cat = poi?.category ?? feature.categoryIcon;
      const city = poi?.tags?.["addr:city"];
      if (cat && city) return `${humanise(cat)} · ${city}`;
      if (cat) return humanise(cat);
      if (city) return city;
      return "";
    }
    // Plain point: city + country line from reverse-geocode address.
    const addr = reverse.data?.result.address;
    if (addr) {
      const city = addr["city"] ?? addr["town"] ?? addr["village"];
      const country = addr["country"];
      return [city, country].filter(Boolean).join(", ");
    }
    return "";
  }, [feature, poiQuery.data, reverse.data]);

  const hours = feature?.hours ?? poiQuery.data?.tags?.["opening_hours"];
  const phone = feature?.phone ?? poiQuery.data?.tags?.["phone"];
  const website =
    feature?.website ??
    poiQuery.data?.tags?.["website"] ??
    poiQuery.data?.tags?.["contact:website"];

  if (!feature) return null;

  const isPoi = feature.kind === "poi";
  const showPoiChips = isPoi && (hours || phone || website);

  return (
    <div
      role="dialog"
      aria-label={
        isPoi
          ? `Details for ${primary || "place"}`
          : "Dropped pin details"
      }
      className={cn(
        // Match 7.png exactly: rounded-xl card, thumbnail with overlay
        // on the left, name + subtitle + coords-as-blue-link in the
        // middle, round Directions/Share buttons on the right, X in
        // the top-right corner. Same shape for point + POI; POI gets
        // an extra hours/phone/website row when data is available.
        "chrome-surface-lg pointer-events-auto relative flex w-[min(380px,calc(100vw-2rem))] items-stretch gap-2 rounded-xl p-1.5 pr-2 animate-in fade-in slide-in-from-bottom-2",
        className,
      )}
    >
      {/* Top-right close X (overlays the corner like 7.png). */}
      <button
        type="button"
        onClick={onClose}
        aria-label="Close info card"
        className="absolute right-1.5 top-1.5 inline-flex h-5 w-5 items-center justify-center rounded-full text-neutral-500 hover:bg-neutral-100 dark:hover:bg-neutral-800"
      >
        <X className="h-3.5 w-3.5" aria-hidden="true" />
      </button>

      <Thumbnail
        lat={feature.lat}
        lon={feature.lon}
        label={primary}
      />

      {/* Text column: name pinned to TOP of thumbnail; coords pinned to
          BOTTOM with a thin divider above. justify-between + an empty
          spacer is what lines them up to the thumbnail's edges. */}
      <div className="flex min-w-0 flex-1 flex-col justify-between py-0.5 pr-6">
        <div className="flex min-w-0 flex-col gap-0.5">
          <h2 className="truncate text-sm font-semibold leading-tight text-foreground">
            {primary || "Unnamed place"}
          </h2>
          {isPoi && secondary && (
            <p className="truncate text-[11px] leading-tight text-muted-foreground">
              {secondary}
            </p>
          )}
        </div>
        {/* Thin divider + coords link aligned with the thumbnail bottom. */}
        <a
          href={`https://www.openstreetmap.org/?mlat=${feature.lat}&mlon=${feature.lon}#map=17/${feature.lat}/${feature.lon}`}
          target="_blank"
          rel="noreferrer noopener"
          className="block truncate border-t border-black/5 pt-1 text-[11px] tabular-nums text-blue-600 hover:underline dark:border-white/10"
        >
          {feature.lat.toFixed(5)}, {feature.lon.toFixed(5)}
        </a>

        {/* POI extras row (only shown for POI variant when data exists). */}
        {showPoiChips && (
          <div className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-0.5 text-[11px] text-muted-foreground">
            {hours && (
              <span className="inline-flex items-center gap-1">
                <Clock className="h-3 w-3" aria-hidden="true" />
                <span className="truncate max-w-[160px]">{hours}</span>
              </span>
            )}
            {phone && (
              <a
                href={`tel:${phone.replace(/\s+/g, "")}`}
                className="inline-flex items-center gap-1 hover:text-foreground"
              >
                <Phone className="h-3 w-3" aria-hidden="true" />
                <span className="truncate max-w-[140px]">{phone}</span>
              </a>
            )}
            {website && (
              <a
                href={website}
                target="_blank"
                rel="noreferrer noopener"
                className="inline-flex items-center gap-1 hover:text-foreground"
              >
                <Globe className="h-3 w-3" aria-hidden="true" />
                <span className="truncate max-w-[140px]">
                  {displayHost(website)}
                </span>
              </a>
            )}
          </div>
        )}
      </div>

      {/* Right-side action stack: smaller (36px) circles, tighter gap. */}
      <div className="flex shrink-0 items-center gap-1.5 self-center">
        <button
          type="button"
          onClick={() => onDirections?.(feature)}
          aria-label={`Directions to ${primary || "selected point"}`}
          className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-blue-600 text-white shadow transition-colors hover:bg-blue-700 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <Navigation2 className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
        <button
          type="button"
          onClick={() => {
            void handleShare(feature);
          }}
          aria-label="Share this location"
          title="Share this location"
          className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-blue-50 text-blue-700 transition-colors hover:bg-blue-100 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring dark:bg-blue-900/30 dark:text-blue-300"
        >
          <Share2 className="h-3.5 w-3.5" aria-hidden="true" />
        </button>
        <ShareDialog open={shareOpen} onOpenChange={setShareOpen} />
      </div>
    </div>
  );
}

/**
 * 80×80 preview image. Tries the server-side `/og/preview.png` endpoint
 * first; falls back to a gradient tile with the first two letters of
 * the label when the image fails (no region installed, offline, etc.).
 */
function Thumbnail({
  lat,
  lon,
  label,
}: {
  lat: number;
  lon: number;
  label: string;
}) {
  // Coords are quantized to ~50m so panning around the same building
  // hits the same image URL → browser cache absorbs it → ~zero OG
  // requests after first click in an area. This is what kills the 429s.
  const qLat = lat.toFixed(3);
  const qLon = lon.toFixed(3);
  const url = apiUrl(
    `/og/preview.png?lat=${qLat}&lon=${qLon}&zoom=15&width=320&height=320`,
  );
  const initials = (label || "??")
    .trim()
    .slice(0, 2)
    .toUpperCase();
  return (
    <div className="relative h-[64px] w-[80px] shrink-0 overflow-hidden rounded-md bg-gradient-to-br from-blue-500 to-indigo-600">
      {/* Placeholder initials sit behind the img so a broken/missing
          fetch leaves the gradient + letters visible. */}
      <span className="absolute inset-0 flex items-center justify-center text-lg font-semibold text-white/90">
        {initials}
      </span>
      <img
        src={url}
        alt=""
        aria-hidden="true"
        loading="lazy"
        className="absolute inset-0 h-full w-full object-cover"
        onError={(e) => {
          (e.currentTarget as HTMLImageElement).style.display = "none";
        }}
      />
      {/* Bottom overlay label like 7.png's "Street View" badge — we
          have no Street View, but the small pin/360 motif keeps the
          card visually identical. */}
      <span className="absolute inset-x-0 bottom-0 flex items-center gap-1 bg-gradient-to-t from-black/70 to-transparent px-2 pb-1 pt-3 text-[10px] font-medium text-white">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
          className="h-3 w-3"
          aria-hidden="true"
        >
          <circle cx="12" cy="12" r="10" />
          <path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z" />
        </svg>
        Map View
      </span>
    </div>
  );
}

function humanise(s: string): string {
  return s
    .replace(/[._:-]+/g, " ")
    .replace(/\s+/g, " ")
    .trim()
    .replace(/\b\w/g, (c) => c.toUpperCase());
}

function displayHost(url: string): string {
  try {
    const u = new URL(url);
    return u.host.replace(/^www\./, "");
  } catch {
    return url;
  }
}

/**
 * Build the deep-link URL the share button publishes:
 *
 *   https://<host>/?lat=<lat>&lon=<lon>&zoom=15
 *
 * POI features tack on `&place=<id>` so opening the link re-selects the
 * same POI rather than dropping a plain pin at the coords. The
 * companion decoder (`decodeURL`) reads back the same keys; both live
 * here together so adding a field stays in sync.
 */
export function buildShareLink(host: string, feature: SelectedFeature): string {
  const params = new URLSearchParams();
  params.set("lat", feature.lat.toFixed(5));
  params.set("lon", feature.lon.toFixed(5));
  params.set("zoom", "15");
  if (feature.kind === "poi" && feature.id) {
    params.set("place", feature.id);
  }
  const proto =
    typeof window !== "undefined" && window.location?.protocol
      ? window.location.protocol
      : "https:";
  return `${proto}//${host}/?${params.toString()}`;
}

/**
 * Default handler for the directions CTA: writes the feature into the
 * directions store's "destination" slot and flips the left rail to
 * the directions tab. Exported so tests can exercise the wiring and
 * other callers can reuse the same flow.
 */
export function useDefaultDirectionsAction(): (f: SelectedFeature) => void {
  const setEndFromPoint = useDirectionsStore((s) => s.setEndFromPoint);
  const openLeftRail = useMapStore((s) => s.openLeftRail);
  return (feature) => {
    setEndFromPoint(
      { lng: feature.lon, lat: feature.lat },
      feature.name,
    );
    openLeftRail("directions");
  };
}

/**
 * Convenience hook that reads the active feature straight from the
 * place store and provides the default close + directions actions.
 * Rendered by `app/page.tsx` so the card can be dropped into the
 * chrome overlay with no prop wiring.
 */
export function PointInfoCardHost() {
  const feature = usePlaceStore((s) => s.selectedFeature);
  const clearSelectedFeature = usePlaceStore((s) => s.clearSelectedFeature);
  const clearPendingClick = useMapStore((s) => s.clearPendingClick);
  const map = useMapStore((s) => s.map);
  const categorySearchResults = useMapStore((s) => s.categorySearchResults);
  const directionsAction = useDefaultDirectionsAction();

  // When the selected feature is a POI that's already drawn as a chip
  // marker (PoiSearchChips owns those markers), skip the dropped-pin
  // overlay so the user doesn't see the teardrop stacked on top of
  // the dark-grey pin (audit F13).
  const suppressDroppedPin = useMemo(() => {
    if (!feature || feature.kind !== "poi" || !feature.id) return false;
    return categorySearchResults.some((p) => p.id === feature.id);
  }, [feature, categorySearchResults]);

  // Drop the Google-Maps-style "dropped pin" marker at the feature's
  // location while the card is visible. Creating the marker here
  // (rather than in MapCanvas) keeps lifecycle colocated with the
  // card: the marker is added when a feature appears and removed as
  // soon as it goes away or the host unmounts. The pin is a small
  // dark-grey badge with a tiny white pin glyph — see
  // `makeDroppedPinElement` for the SVG source.
  useEffect(() => {
    if (!map || !feature) return;
    if (suppressDroppedPin) return;
    let marker: maplibregl.Marker | null = null;
    try {
      const el = makeDroppedPinElement();
      marker = new maplibregl.Marker({ element: el, anchor: "bottom" })
        .setLngLat([feature.lon, feature.lat])
        .addTo(map);
    } catch {
      // jsdom / test environments don't implement every MapLibre API;
      // bail quietly so the rest of the card still renders.
      marker = null;
    }
    return () => {
      if (marker) {
        try {
          marker.remove();
        } catch {
          /* ignore — the map may have been torn down already. */
        }
      }
    };
  }, [map, feature, suppressDroppedPin]);

  return (
    <PointInfoCard
      feature={feature}
      onClose={() => {
        clearSelectedFeature();
        clearPendingClick();
      }}
      onDirections={(f) => {
        directionsAction(f);
      }}
    />
  );
}
