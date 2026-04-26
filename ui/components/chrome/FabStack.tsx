"use client";

import { useEffect, useRef } from "react";
import maplibregl from "maplibre-gl";
import { Compass, LocateFixed, Minus, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import { ToolsFab } from "@/components/tools/ToolsFab";
import { useMapStore } from "@/lib/state/map";

export interface FabStackProps {
  onZoomIn?: () => void;
  onZoomOut?: () => void;
  onResetBearing?: () => void;
  onLocate?: () => void;
  /** Toggle for the Phase-7 tool suite FAB (measure + isochrone). */
  showTools?: boolean;
}

/**
 * Bottom-right floating action buttons — sized to match Google Maps'
 * desktop chrome: 32×32 square buttons, `rounded-lg`, `shadow-sm`, and
 * a single vertical pill for the zoom + / − pair (32×64 with a 1px
 * divider between them). 4px gap between adjacent cards. A 16px icon.
 *
 * With MapLibre's native NavigationControl + GeolocateControl suppressed
 * (see `MainMap`), this is the sole right-rail stack — so the default
 * handlers talk to the live map instance via the Zustand store.
 */
export function FabStack({
  onZoomIn,
  onZoomOut,
  onResetBearing,
  onLocate,
  showTools = true,
}: FabStackProps) {
  // Pull the live map once — Zustand subscribers re-render when it flips
  // from null → MapLibreMap, so the defaults kick in as soon as the
  // canvas is ready.
  const map = useMapStore((s) => s.map);

  // Tracks the blue GPS-position marker we drop when the user clicks
  // "Show my location". Kept separate from the red click-pin marker so
  // both can coexist (Google shows them side-by-side).
  const locationMarkerRef = useRef<maplibregl.Marker | null>(null);

  // Tidy the location marker on unmount — otherwise a remount would
  // orphan the DOM element on the map.
  useEffect(() => {
    return () => {
      if (locationMarkerRef.current) {
        try {
          locationMarkerRef.current.remove();
        } catch {
          /* ignore */
        }
        locationMarkerRef.current = null;
      }
    };
  }, []);

  const handleZoomIn = onZoomIn ?? (() => map?.zoomIn());
  const handleZoomOut = onZoomOut ?? (() => map?.zoomOut());
  const handleResetBearing =
    onResetBearing ??
    (() => map?.easeTo({ bearing: 0, pitch: 0, duration: 300 }));
  const handleLocate =
    onLocate ??
    (() => {
      if (!map || typeof navigator === "undefined" || !navigator.geolocation)
        return;
      navigator.geolocation.getCurrentPosition(
        (pos) => {
          const lon = pos.coords.longitude;
          const lat = pos.coords.latitude;
          map.easeTo({
            center: [lon, lat],
            zoom: Math.max(map.getZoom(), 15),
            duration: 600,
          });
          // Replace any previous GPS marker, then drop a new one at
          // the resolved position. Blue dot + white border + faint
          // pulsing accuracy ring, Google Maps style.
          if (locationMarkerRef.current) {
            try {
              locationMarkerRef.current.remove();
            } catch {
              /* ignore */
            }
            locationMarkerRef.current = null;
          }
          try {
            const el = document.createElement("div");
            el.setAttribute("aria-hidden", "true");
            el.className =
              "w-4 h-4 rounded-full bg-blue-500 border-2 border-white shadow-[0_0_0_8px_rgba(66,133,244,0.15)]";
            locationMarkerRef.current = new maplibregl.Marker({
              element: el,
              anchor: "center",
            })
              .setLngLat([lon, lat])
              .addTo(map);
          } catch {
            /* ignore — map may have torn down between request + resolve. */
          }
        },
        () => {
          /* Silently ignore — the FAB only offers a courtesy jump. */
        },
        { enableHighAccuracy: true, timeout: 10_000 },
      );
    });

  return (
    <div className="pointer-events-auto flex flex-col items-end gap-1">
      {showTools && <ToolsFab />}
      <FabButton
        onClick={handleResetBearing}
        ariaLabel="Reset bearing to north"
      >
        <Compass className="h-4 w-4" aria-hidden="true" />
      </FabButton>

      {/* Combined zoom pill — one 32×64 card with a 1-px divider. */}
      <div
        className={cn(
          "chrome-surface-sm flex flex-col overflow-hidden rounded-lg",
        )}
      >
        <button
          type="button"
          onClick={handleZoomIn}
          aria-label="Zoom in"
          title="Zoom in"
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center",
            "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
        </button>
        <div className="mx-1.5 h-px bg-chrome-border" aria-hidden="true" />
        <button
          type="button"
          onClick={handleZoomOut}
          aria-label="Zoom out"
          title="Zoom out"
          className={cn(
            "inline-flex h-8 w-8 items-center justify-center",
            "hover:bg-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
          )}
        >
          <Minus className="h-4 w-4" aria-hidden="true" />
        </button>
      </div>

      <FabButton onClick={handleLocate} ariaLabel="Show my location">
        <LocateFixed className="h-4 w-4" aria-hidden="true" />
      </FabButton>
    </div>
  );
}

/**
 * Shared 32×32 floating card button: white surface, subtle black/10
 * ring, `shadow-sm`, `rounded-lg`. Internal helper so the compass /
 * locate / (and future single-icon FABs) share a single style source.
 */
function FabButton({
  onClick,
  ariaLabel,
  children,
}: {
  onClick?: () => void;
  ariaLabel: string;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={ariaLabel}
      title={ariaLabel}
      className={cn(
        "chrome-surface-sm inline-flex h-8 w-8 items-center justify-center rounded-lg hover:bg-muted",
        "focus:outline-none focus-visible:ring-2 focus-visible:ring-ring",
      )}
    >
      {children}
    </button>
  );
}
