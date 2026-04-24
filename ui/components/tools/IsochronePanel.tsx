"use client";

import { useCallback } from "react";
import { X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useIsochrone } from "@/lib/api/hooks";
import { useMapStore } from "@/lib/state/map";
import { useActiveToolStore } from "@/lib/tools/active-tool";
import {
  AVAILABLE_MINUTES,
  type AvailableMinutes,
  type IsochroneMode,
  useIsochroneStore,
} from "@/lib/tools/isochrone-state";

const MODE_OPTIONS: Array<{ value: IsochroneMode; label: string }> = [
  { value: "auto", label: "Driving" },
  { value: "bicycle", label: "Cycling" },
  { value: "pedestrian", label: "Walking" },
];

/**
 * Config panel for the isochrone tool. Appears only when the isochrone
 * tool is active. Fires `POST /api/isochrone` via `useIsochrone` on
 * Render. The map click itself is captured by `IsochroneTool` into
 * `origin`; the panel falls back to the current map centre when the
 * user clicks Render without clicking the map first.
 */
export function IsochronePanel() {
  const active = useActiveToolStore((s) => s.active);
  const closeAll = useActiveToolStore((s) => s.closeAll);

  const map = useMapStore((s) => s.map);
  const origin = useIsochroneStore((s) => s.origin);
  const setOrigin = useIsochroneStore((s) => s.setOrigin);
  const mode = useIsochroneStore((s) => s.mode);
  const setMode = useIsochroneStore((s) => s.setMode);
  const minutes = useIsochroneStore((s) => s.minutes);
  const toggleMinutes = useIsochroneStore((s) => s.toggleMinutes);
  const setResult = useIsochroneStore((s) => s.setResult);
  const setLoading = useIsochroneStore((s) => s.setLoading);
  const result = useIsochroneStore((s) => s.result);

  const isochrone = useIsochrone();

  const resolveOrigin = useCallback(() => {
    if (origin) return origin;
    if (map) {
      const c = map.getCenter();
      return { lng: c.lng, lat: c.lat };
    }
    return null;
  }, [origin, map]);

  const onRender = useCallback(() => {
    const o = resolveOrigin();
    if (!o || minutes.length === 0) return;
    setLoading(true);
    // Persist resolved origin so the subsequent "Clear" knows where it
    // came from and tests can assert deterministic state.
    setOrigin(o);
    isochrone.mutate(
      {
        origin: { lat: o.lat, lon: o.lng },
        mode,
        contoursSeconds: minutes.map((m) => m * 60),
      },
      {
        onSuccess: (data) => {
          setResult(data);
          setLoading(false);
        },
        onError: () => setLoading(false),
      },
    );
  }, [
    resolveOrigin,
    minutes,
    mode,
    setLoading,
    setOrigin,
    isochrone,
    setResult,
  ]);

  if (active !== "isochrone") return null;

  return (
    <div
      className="pointer-events-auto chrome-card absolute right-20 top-4 z-20 w-72 p-4 text-sm shadow-lg"
      role="dialog"
      aria-label="Isochrone tool"
    >
      <div className="flex items-center justify-between">
        <div className="font-semibold">Travel time</div>
        <Button
          variant="ghost"
          size="sm"
          onClick={closeAll}
          aria-label="Close isochrone tool"
        >
          <X className="h-4 w-4" aria-hidden="true" />
        </Button>
      </div>

      <div className="mt-3 text-xs text-muted-foreground">
        {origin
          ? `Origin ${origin.lat.toFixed(4)}, ${origin.lng.toFixed(4)}`
          : "Click the map to set origin (or use map centre)"}
      </div>

      <fieldset className="mt-3">
        <legend className="mb-1 text-xs font-medium">Mode</legend>
        <div className="flex gap-1" role="radiogroup" aria-label="Travel mode">
          {MODE_OPTIONS.map((opt) => (
            <button
              key={opt.value}
              type="button"
              role="radio"
              aria-checked={mode === opt.value}
              onClick={() => setMode(opt.value)}
              className={
                mode === opt.value
                  ? "rounded-md bg-primary px-3 py-1 text-xs text-primary-foreground"
                  : "rounded-md border border-border px-3 py-1 text-xs hover:bg-muted"
              }
            >
              {opt.label}
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset className="mt-3">
        <legend className="mb-1 text-xs font-medium">Contours</legend>
        <div className="flex flex-wrap gap-2">
          {AVAILABLE_MINUTES.map((m) => (
            <label key={m} className="flex items-center gap-1 text-xs">
              <input
                type="checkbox"
                checked={minutes.includes(m as AvailableMinutes)}
                onChange={() => toggleMinutes(m as AvailableMinutes)}
              />
              {m} min
            </label>
          ))}
        </div>
      </fieldset>

      <div className="mt-4 flex items-center justify-between gap-2">
        <Button
          variant="secondary"
          size="sm"
          onClick={() => setResult(null)}
          disabled={!result}
        >
          Clear
        </Button>
        <Button
          onClick={onRender}
          size="sm"
          disabled={isochrone.isPending || minutes.length === 0}
        >
          {isochrone.isPending ? "Rendering…" : "Render"}
        </Button>
      </div>

      <div className="sr-only" role="status" aria-live="polite">
        {isochrone.isPending
          ? "Computing travel-time polygons…"
          : result
            ? `Rendered ${result.features.length} contour bands.`
            : ""}
      </div>
    </div>
  );
}
